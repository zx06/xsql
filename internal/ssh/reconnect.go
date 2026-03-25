package ssh

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// StatusType describes the kind of status event emitted by ReconnectDialer.
type StatusType int

const (
	StatusConnected       StatusType = iota // initial connection established
	StatusDisconnected                      // connection detected as dead
	StatusReconnecting                      // reconnection attempt in progress
	StatusReconnected                       // reconnection succeeded
	StatusReconnectFailed                   // reconnection attempt failed
)

// StatusEvent is emitted by ReconnectDialer when connection state changes.
type StatusEvent struct {
	Type    StatusType
	Message string
	Error   error
}

// ReconnectDialer wraps SSH Connect with automatic reconnection and keepalive.
// It implements the same DialContext/Close interface as *Client, making it a
// drop-in replacement wherever a Dialer is expected (e.g. proxy.Dialer).
type ReconnectDialer struct {
	mu     sync.Mutex
	client *Client
	opts   Options
	closed bool

	ctx    context.Context
	cancel context.CancelFunc

	keepaliveCancel context.CancelFunc

	onStatus func(StatusEvent)

	// connectFunc allows injecting a custom connect function for testing.
	connectFunc func(ctx context.Context, opts Options) (*Client, error)

	// Reconnect coalescing: when multiple goroutines detect failure simultaneously,
	// only one performs the actual reconnect; others wait for the result.
	reconnecting bool
	reconnectCh  chan struct{}
}

// ReconnectOption configures a ReconnectDialer.
type ReconnectOption func(*ReconnectDialer)

// WithStatusCallback sets a callback for connection status events.
func WithStatusCallback(fn func(StatusEvent)) ReconnectOption {
	return func(rd *ReconnectDialer) {
		rd.onStatus = fn
	}
}

// NewReconnectDialer creates a ReconnectDialer that automatically reconnects
// on SSH connection failures. It establishes the initial connection and starts
// keepalive monitoring.
func NewReconnectDialer(ctx context.Context, opts Options, ropts ...ReconnectOption) (*ReconnectDialer, error) {
	rdCtx, cancel := context.WithCancel(ctx)

	rd := &ReconnectDialer{
		opts:   opts,
		ctx:    rdCtx,
		cancel: cancel,
	}

	for _, o := range ropts {
		o(rd)
	}

	client, err := rd.connect(rdCtx, opts)
	if err != nil {
		cancel()
		return nil, err
	}
	rd.client = client
	rd.emitStatus(StatusConnected, "ssh connection established", nil)

	rd.startKeepalive()

	return rd, nil
}

// DialContext dials the remote address through the SSH tunnel.
// If the dial fails, it attempts to reconnect and retry once.
func (rd *ReconnectDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	rd.mu.Lock()
	if rd.closed {
		rd.mu.Unlock()
		return nil, fmt.Errorf("reconnect dialer is closed")
	}
	client := rd.client
	rd.mu.Unlock()

	if client == nil {
		// Client was cleared by a concurrent reconnect; wait for it.
		newClient, reconnErr := rd.reconnect()
		if reconnErr != nil {
			return nil, fmt.Errorf("no active connection and reconnect failed: %v", reconnErr)
		}
		return newClient.DialContext(ctx, network, addr)
	}

	conn, err := client.DialContext(ctx, network, addr)
	if err == nil {
		return conn, nil
	}

	// Dial failed — attempt reconnect (coalesced with other callers)
	newClient, reconnErr := rd.reconnect()
	if reconnErr != nil {
		return nil, fmt.Errorf("dial failed (%v) and reconnect failed (%v)", err, reconnErr)
	}

	// Retry with new client
	return newClient.DialContext(ctx, network, addr)
}

// Close shuts down the dialer, stops keepalive, and closes the SSH connection.
func (rd *ReconnectDialer) Close() error {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	if rd.closed {
		return nil
	}
	rd.closed = true
	rd.cancel()

	if rd.keepaliveCancel != nil {
		rd.keepaliveCancel()
	}

	if rd.client != nil {
		return rd.client.Close()
	}
	return nil
}

// reconnect closes the current client and establishes a new SSH connection.
// Concurrent callers are coalesced: the first caller performs the reconnect,
// others wait for its result. The lock is NOT held during the actual retry loop
// to avoid blocking DialContext callers.
func (rd *ReconnectDialer) reconnect() (*Client, error) {
	rd.mu.Lock()

	if rd.closed {
		rd.mu.Unlock()
		return nil, fmt.Errorf("reconnect dialer is closed")
	}

	// Coalescing: if another goroutine is already reconnecting, wait for it.
	if rd.reconnecting {
		ch := rd.reconnectCh
		rd.mu.Unlock()
		<-ch
		// Check the result
		rd.mu.Lock()
		client := rd.client
		rd.mu.Unlock()
		if client != nil {
			return client, nil
		}
		return nil, fmt.Errorf("reconnection by another goroutine failed")
	}

	// This goroutine wins: mark as reconnecting.
	rd.reconnecting = true
	rd.reconnectCh = make(chan struct{})

	rd.emitStatus(StatusReconnecting, "attempting ssh reconnection", nil)

	// Stop old keepalive
	if rd.keepaliveCancel != nil {
		rd.keepaliveCancel()
		rd.keepaliveCancel = nil
	}

	// Close old client
	if rd.client != nil {
		_ = rd.client.Close()
		rd.client = nil
	}

	// Release lock during retry loop to avoid blocking DialContext callers.
	rd.mu.Unlock()

	// Attempt reconnection with retries (lock-free)
	var newClient *Client
	var lastErr error
	maxRetries := rd.opts.keepaliveCountMax()
	for i := range maxRetries {
		select {
		case <-rd.ctx.Done():
			rd.finishReconnect(nil)
			return nil, rd.ctx.Err()
		default:
		}

		client, err := rd.connect(rd.ctx, rd.opts)
		if err == nil {
			newClient = client
			break
		}
		lastErr = err
		rd.emitStatus(StatusReconnectFailed,
			fmt.Sprintf("reconnect attempt %d/%d failed", i+1, maxRetries), err)

		// Brief exponential backoff between retries
		select {
		case <-rd.ctx.Done():
			rd.finishReconnect(nil)
			return nil, rd.ctx.Err()
		case <-time.After(time.Duration(i+1) * time.Second):
		}
	}

	// Re-acquire lock to update state
	rd.finishReconnect(newClient)

	if newClient == nil {
		return nil, fmt.Errorf("reconnection failed after %d attempts: %w", maxRetries, lastErr)
	}
	return newClient, nil
}

// finishReconnect updates state after a reconnect attempt completes.
// It sets the new client, restarts keepalive, and unblocks waiting goroutines.
func (rd *ReconnectDialer) finishReconnect(newClient *Client) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	if newClient != nil {
		rd.client = newClient
		rd.emitStatus(StatusReconnected, "ssh reconnected successfully", nil)
	}

	// Always restart keepalive so health monitoring continues even after failure.
	// On success, it monitors the new connection; on failure, it will detect
	// the nil/dead client and trigger another reconnect attempt.
	if !rd.closed {
		rd.startKeepaliveLocked()
	}

	rd.reconnecting = false
	close(rd.reconnectCh)
}

// startKeepalive starts the keepalive monitor (caller must NOT hold mu).
func (rd *ReconnectDialer) startKeepalive() {
	rd.mu.Lock()
	defer rd.mu.Unlock()
	rd.startKeepaliveLocked()
}

// startKeepaliveLocked starts the keepalive monitor (caller MUST hold mu).
func (rd *ReconnectDialer) startKeepaliveLocked() {
	interval := rd.opts.keepaliveInterval()
	maxMissed := rd.opts.keepaliveCountMax()

	if interval <= 0 {
		return
	}

	// Stop any existing keepalive before starting a new one.
	if rd.keepaliveCancel != nil {
		rd.keepaliveCancel()
	}

	kaCtx, kaCancel := context.WithCancel(rd.ctx)
	rd.keepaliveCancel = kaCancel

	go rd.keepaliveLoop(kaCtx, interval, maxMissed)
}

// keepaliveLoop runs periodic keepalive checks and triggers reconnection on failure.
func (rd *ReconnectDialer) keepaliveLoop(ctx context.Context, interval time.Duration, maxMissed int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	missed := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rd.mu.Lock()
			client := rd.client
			closed := rd.closed
			rd.mu.Unlock()

			if closed || client == nil {
				return
			}

			if err := client.SendKeepalive(); err != nil {
				missed++
				if missed >= maxMissed {
					rd.emitStatus(StatusDisconnected,
						fmt.Sprintf("keepalive failed %d consecutive times", missed), err)
					// Trigger reconnection (coalesced with any concurrent callers)
					go func() {
						if _, reconnErr := rd.reconnect(); reconnErr != nil {
							log.Printf("[ssh] keepalive-triggered reconnect failed: %v", reconnErr)
						}
					}()
					return
				}
			} else {
				missed = 0
			}
		}
	}
}

// connect calls the configured connect function or the default Connect.
func (rd *ReconnectDialer) connect(ctx context.Context, opts Options) (*Client, error) {
	if rd.connectFunc != nil {
		return rd.connectFunc(ctx, opts)
	}
	client, xe := Connect(ctx, opts)
	if xe != nil {
		return nil, xe
	}
	return client, nil
}

// emitStatus sends a status event if a callback is configured.
func (rd *ReconnectDialer) emitStatus(t StatusType, msg string, err error) {
	if rd.onStatus != nil {
		rd.onStatus(StatusEvent{Type: t, Message: msg, Error: err})
	}
}


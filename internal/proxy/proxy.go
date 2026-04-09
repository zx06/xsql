package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/zx06/xsql/internal/errors"
)

// Dialer defines the interface for establishing connections through SSH.
// This allows testing with mock implementations.
type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	Close() error
}

// Proxy represents a TCP port forwarding proxy.
type Proxy struct {
	dialer   Dialer
	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// Options holds configuration for starting a proxy.
type Options struct {
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Dialer     Dialer
}

// Result contains information about a started proxy.
type Result struct {
	LocalAddress  string
	RemoteAddress string
	SSHProxy      string
}

// Start creates and starts a TCP port forwarding proxy.
func Start(ctx context.Context, opts Options) (*Proxy, *Result, *errors.XError) {
	if opts.LocalHost == "" {
		opts.LocalHost = "127.0.0.1"
	}
	if opts.Dialer == nil {
		return nil, nil, errors.New(errors.CodeInternal, "dialer is required", nil)
	}

	// Start listening on local port
	addr := fmt.Sprintf("%s:%d", opts.LocalHost, opts.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Check if this is a port-in-use error
		if isPortInUse(err) {
			return nil, nil, errors.New(errors.CodePortInUse, "port is already in use",
				map[string]any{"address": addr, "port": opts.LocalPort})
		}
		return nil, nil, errors.Wrap(errors.CodeInternal, "failed to listen on local port", map[string]any{"address": addr}, err)
	}

	proxyCtx, cancel := context.WithCancel(ctx)

	// Get actual local address (in case port was auto-assigned)
	localAddr := listener.Addr().String()
	remoteAddr := fmt.Sprintf("%s:%d", opts.RemoteHost, opts.RemotePort)

	// Build SSH proxy description for result
	sshProxy := fmt.Sprintf("%s:%d", opts.RemoteHost, opts.RemotePort)

	p := &Proxy{
		dialer:   opts.Dialer,
		listener: listener,
		ctx:      proxyCtx,
		cancel:   cancel,
	}

	// Start accepting connections
	p.wg.Add(1)
	go p.acceptConnections(opts.RemoteHost, opts.RemotePort)

	result := &Result{
		LocalAddress:  localAddr,
		RemoteAddress: remoteAddr,
		SSHProxy:      sshProxy,
	}

	return p, result, nil
}

// acceptConnections accepts incoming connections and forwards them.
func (p *Proxy) acceptConnections(remoteHost string, remotePort int) {
	defer p.wg.Done()

	remoteAddr := fmt.Sprintf("%s:%d", remoteHost, remotePort)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			localConn, err := p.listener.Accept()
			if err != nil {
				// Check if context was canceled
				select {
				case <-p.ctx.Done():
					return
				default:
					// Log error but continue accepting
					continue
				}
			}

			p.wg.Add(1)
			go p.handleConnection(localConn, remoteAddr)
		}
	}
}

// handleConnection handles a single connection by forwarding it through SSH.
func (p *Proxy) handleConnection(localConn net.Conn, remoteAddr string) {
	defer p.wg.Done()
	defer func() {
		if localConn != nil {
			_ = localConn.Close()
		}
	}()

	// Dial remote through SSH
	remoteConn, err := p.dialer.DialContext(p.ctx, "tcp", remoteAddr)
	if err != nil {
		log.Printf("[proxy] failed to dial remote %s: %v", remoteAddr, err)
		return
	}
	if remoteConn == nil {
		log.Printf("[proxy] dial returned nil conn")
		return
	}
	if remoteConn != nil {
		defer func() {
			if closeErr := remoteConn.Close(); closeErr != nil {
				log.Printf("[proxy] failed to close remote connection: %v", closeErr)
			}
		}()
	}

	// Bidirectional copy
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(localConn, remoteConn); err != nil {
			errChan <- fmt.Errorf("copy remote->local failed: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(remoteConn, localConn); err != nil {
			errChan <- fmt.Errorf("copy local->remote failed: %w", err)
		}
	}()

	// Wait for both copies to finish or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Check if there were any copy errors
		select {
		case err := <-errChan:
			log.Printf("[proxy] connection copy error: %v", err)
		default:
		}
	case <-p.ctx.Done():
		// Context cancelled: close connections to unblock io.Copy goroutines
		_ = localConn.Close()
		_ = remoteConn.Close()
		// Wait for goroutines to finish
		<-done
		// Check for any final errors
		select {
		case err := <-errChan:
			log.Printf("[proxy] connection copy error on shutdown: %v", err)
		default:
		}
	}
}

// Stop gracefully shuts down the proxy.
func (p *Proxy) Stop() error {
	p.cancel()
	var err error
	if p.listener != nil {
		err = p.listener.Close()
	} else {
		log.Printf("[proxy] listener is nil during stop")
	}
	p.wg.Wait()
	return err
}

// LocalAddress returns the actual local address the proxy is listening on.
func (p *Proxy) LocalAddress() string {
	if p.listener != nil {
		return p.listener.Addr().String()
	}
	return ""
}

// IsPortAvailable checks if a port is available for binding.
func IsPortAvailable(host string, port int) bool {
	if host == "" {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// isPortInUse checks if the error is caused by the port being in use.
func isPortInUse(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return contains(s, "address already in use") || contains(s, "bind: address already in use") ||
		contains(s, "Only one usage of each socket address")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package proxy

import (
	"context"
	"fmt"
	"io"
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
	dialer    Dialer
	listener  net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
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
	defer func() { _ = localConn.Close() }()

	// Dial remote through SSH
	remoteConn, err := p.dialer.DialContext(p.ctx, "tcp", remoteAddr)
	if err != nil {
		return
	}
	defer func() { _ = remoteConn.Close() }()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(localConn, remoteConn)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(remoteConn, localConn)
	}()

	// Wait for both copies to finish or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-p.ctx.Done():
	}
}

// Stop gracefully shuts down the proxy.
func (p *Proxy) Stop() error {
	p.cancel()
	err := p.listener.Close()
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
package proxy

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/errors"
)

// mockSSHClient implements a minimal SSH client for testing.
type mockSSHClient struct {
	remoteAddr string
	listener   net.Listener
}

func newMockSSHClient(t *testing.T, remoteAddr string) *mockSSHClient {
	listener, err := net.Listen("tcp", remoteAddr)
	if err != nil {
		t.Fatalf("failed to create mock listener: %v", err)
	}
	return &mockSSHClient{
		remoteAddr: remoteAddr,
		listener:   listener,
	}
}

func (m *mockSSHClient) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if addr != m.remoteAddr {
		return nil, &net.OpError{Op: "dial", Err: &net.AddrError{Err: "connection refused", Addr: addr}}
	}
	return net.Dial("tcp", m.listener.Addr().String())
}

func (m *mockSSHClient) Close() error {
	return m.listener.Close()
}

func TestProxyStart(t *testing.T) {
	tests := []struct {
		name        string
		opts        Options
		expectError bool
	}{
		{
			name: "start with auto port",
			opts: Options{
				LocalHost:  "127.0.0.1",
				LocalPort:  0, // auto-assign
				RemoteHost: "127.0.0.1",
				RemotePort: 18080,
			},
			expectError: false,
		},
		{
			name: "start with specific port",
			opts: Options{
				LocalHost:  "127.0.0.1",
				LocalPort:  18081,
				RemoteHost: "127.0.0.1",
				RemotePort: 18080,
			},
			expectError: false,
		},
		{
			name: "missing ssh client",
			opts: Options{
				LocalHost:  "127.0.0.1",
				LocalPort:  18082,
				RemoteHost: "127.0.0.1",
				RemotePort: 18080,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			var dialer *mockSSHClient
			if !tt.expectError {
				dialer = newMockSSHClient(t, "127.0.0.1:18080")
				defer func() { _ = dialer.Close() }()
				tt.opts.Dialer = dialer
			}

			proxy, result, xe := Start(ctx, tt.opts)

			if tt.expectError {
				if xe == nil {
					t.Error("expected error but got none")
				}
				if proxy != nil {
					_ = proxy.Stop()
				}
				return
			}

			if xe != nil {
				t.Fatalf("unexpected error: %v", xe)
			}

			if proxy == nil {
				t.Fatal("proxy should not be nil")
			}

			if result == nil {
				t.Fatal("result should not be nil")
			}

			if result.LocalAddress == "" {
				t.Error("local address should not be empty")
			}

			if result.RemoteAddress != "127.0.0.1:18080" {
				t.Errorf("remote address mismatch: got %s, want 127.0.0.1:18080", result.RemoteAddress)
			}

			// Verify the local address is actually listening
			conn, err := net.DialTimeout("tcp", result.LocalAddress, 1*time.Second)
			if err != nil {
				t.Errorf("failed to dial local address: %v", err)
			}
			_ = conn.Close()

			_ = proxy.Stop()
		})
	}
}

func TestProxyStop(t *testing.T) {
	dialer := newMockSSHClient(t, "127.0.0.1:18090")
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  18091,
		RemoteHost: "127.0.0.1",
		RemotePort: 18090,
		Dialer:     dialer,
	}

	proxy, result, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("failed to start proxy: %v", xe)
	}

	// Verify proxy is listening
	conn, err := net.DialTimeout("tcp", result.LocalAddress, 1*time.Second)
	if err != nil {
		t.Errorf("failed to dial local address: %v", err)
	}
	_ = conn.Close()

	// Stop the proxy
	if err := proxy.Stop(); err != nil {
		t.Errorf("failed to stop proxy: %v", err)
	}

	// Verify proxy is no longer listening
	conn, err = net.DialTimeout("tcp", result.LocalAddress, 1*time.Second)
	if err == nil {
		_ = conn.Close()
		t.Error("proxy should not be listening after stop")
	}
}

func TestProxyLocalAddress(t *testing.T) {
	dialer := newMockSSHClient(t, "127.0.0.1:18100")
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  0, // auto-assign
		RemoteHost: "127.0.0.1",
		RemotePort: 18100,
		Dialer:     dialer,
	}

	proxy, _, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("failed to start proxy: %v", xe)
	}
	defer func() { _ = proxy.Stop() }()

	addr := proxy.LocalAddress()
	if addr == "" {
		t.Error("local address should not be empty")
	}

	// Verify address format
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Errorf("invalid address format: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("unexpected host: got %s, want 127.0.0.1", host)
	}
	if port == "0" {
		t.Error("port should not be 0 after starting")
	}
}

// Test integration with actual ssh.Client (minimal test)
func TestProxyWithRealSSHClient(t *testing.T) {
	// This test verifies that our proxy works with the real ssh.Client interface
	// We can't actually test SSH connections in unit tests, but we verify type compatibility

	ctx := context.Background()

	// Create a proxy with nil dialer to test error handling
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  0,
		RemoteHost: "example.com",
		RemotePort: 3306,
		Dialer:     nil, // This should cause an error
	}

	_, _, xe := Start(ctx, opts)
	if xe == nil {
		t.Error("expected error when SSH client is nil")
	}

	// Verify it's an internal error
	if xe != nil {
		if xe.Code != errors.CodeInternal {
			t.Errorf("expected CodeInternal, got %s", xe.Code)
		}
	}
}

func TestProxy_PortInUse(t *testing.T) {
	// Find an available port, bind to it, then try to start proxy on same port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Get the port that was allocated
	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	dialer := newMockSSHClient(t, "127.0.0.1:18200")
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  port,
		RemoteHost: "127.0.0.1",
		RemotePort: 18200,
		Dialer:     dialer,
	}

	_, _, xe := Start(ctx, opts)
	if xe == nil {
		t.Error("expected error when port is already in use")
	}
	if xe != nil && xe.Code != errors.CodePortInUse {
		t.Errorf("expected CodePortInUse, got %s", xe.Code)
	}
}

func TestProxy_DefaultLocalHost(t *testing.T) {
	dialer := newMockSSHClient(t, "127.0.0.1:18300")
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	opts := Options{
		LocalHost:  "", // Should default to 127.0.0.1
		LocalPort:  0,
		RemoteHost: "127.0.0.1",
		RemotePort: 18300,
		Dialer:     dialer,
	}

	proxy, result, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	defer func() { _ = proxy.Stop() }()

	if result.LocalAddress == "" {
		t.Error("local address should not be empty")
	}

	// Verify it's bound to 127.0.0.1
	host, _, err := net.SplitHostPort(result.LocalAddress)
	if err != nil {
		t.Errorf("invalid address format: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("expected default host 127.0.0.1, got %s", host)
	}
}

func TestProxy_ContextCancellation(t *testing.T) {
	dialer := newMockSSHClient(t, "127.0.0.1:18400")
	defer func() { _ = dialer.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  0,
		RemoteHost: "127.0.0.1",
		RemotePort: 18400,
		Dialer:     dialer,
	}

	proxy, _, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	// Cancel the context
	cancel()

	// Stop should complete without hanging
	if err := proxy.Stop(); err != nil {
		t.Errorf("failed to stop proxy: %v", err)
	}
}

type nilConnDialer struct{}

func (d *nilConnDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return nil, nil
}

func (d *nilConnDialer) Close() error { return nil }

func TestProxy_StopAndLocalAddress_WithNilListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Proxy{
		dialer: &nilConnDialer{},
		ctx:    ctx,
		cancel: cancel,
	}

	if got := p.LocalAddress(); got != "" {
		t.Fatalf("expected empty local address, got %q", got)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestProxy_HandleConnection_DialReturnsNilConn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &Proxy{
		dialer: &nilConnDialer{},
		ctx:    ctx,
		cancel: cancel,
	}

	localConn, peer := net.Pipe()
	defer func() { _ = peer.Close() }()

	done := make(chan struct{})
	p.wg.Add(1)
	go func() {
		defer close(done)
		p.handleConnection(localConn, "127.0.0.1:65535")
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConnection should return quickly when dialer returns nil conn")
	}
}

func TestIsPortAvailable(t *testing.T) {
	t.Run("available port", func(t *testing.T) {
		// Port 0 always resolves to an available port; just verify the function works
		// by finding a free port first
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()

		// Port should now be available
		if !IsPortAvailable("127.0.0.1", port) {
			t.Error("port should be available after closing listener")
		}
	})

	t.Run("unavailable port", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()

		port := ln.Addr().(*net.TCPAddr).Port
		if IsPortAvailable("127.0.0.1", port) {
			t.Error("port should not be available while listener is active")
		}
	})

	t.Run("default host", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()

		port := ln.Addr().(*net.TCPAddr).Port
		if IsPortAvailable("", port) {
			t.Error("port should not be available (empty host defaults to 127.0.0.1)")
		}
	})
}

func TestIsPortInUse(t *testing.T) {
	tests := []struct {
		errMsg string
		want   bool
	}{
		{"listen tcp 127.0.0.1:8080: bind: address already in use", true},
		{"address already in use", true},
		{"Only one usage of each socket address", true},
		{"connection refused", false},
		{"", false},
	}

	for _, tt := range tests {
		var err error
		if tt.errMsg != "" {
			err = &net.OpError{Op: "listen", Err: &net.AddrError{Err: tt.errMsg, Addr: "127.0.0.1:8080"}}
		}
		got := isPortInUse(err)
		if got != tt.want {
			t.Errorf("isPortInUse(%q) = %v, want %v", tt.errMsg, got, tt.want)
		}
	}

	// nil error
	if isPortInUse(nil) {
		t.Error("isPortInUse(nil) should return false")
	}
}

func TestProxy_PortInUse_ReturnsCorrectErrorCode(t *testing.T) {
	// Bind to a port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port

	dialer := newMockSSHClient(t, "127.0.0.1:18500")
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	_, _, xe := Start(ctx, Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  port,
		RemoteHost: "127.0.0.1",
		RemotePort: 18500,
		Dialer:     dialer,
	})

	if xe == nil {
		t.Fatal("expected error")
	}
	if xe.Code != errors.CodePortInUse {
		t.Errorf("expected CodePortInUse, got %s", xe.Code)
	}
	if xe.Details == nil {
		t.Fatal("expected details")
	}
	if xe.Details["port"] != port {
		t.Errorf("expected port=%d in details, got %v", port, xe.Details["port"])
	}
}

func TestProxy_HandleConnection_BidirectionalCopy(t *testing.T) {
	// Create a mock echo server as the remote target
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = echoListener.Close() }()

	go func() {
		for {
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					_, _ = c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	echoPort := echoListener.Addr().(*net.TCPAddr).Port

	// Create a dialer that connects to the echo server
	dialer := &directDialer{addr: echoListener.Addr().String()}
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  0,
		RemoteHost: "127.0.0.1",
		RemotePort: echoPort,
		Dialer:     dialer,
	}

	proxy, result, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("failed to start proxy: %v", xe)
	}
	defer func() { _ = proxy.Stop() }()

	// Connect to the proxy and send/receive data
	conn, err := net.DialTimeout("tcp", result.LocalAddress, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer func() { _ = conn.Close() }()

	testData := []byte("hello proxy")
	if _, err := conn.Write(testData); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	buf := make([]byte, len(testData))
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	if string(buf) != string(testData) {
		t.Errorf("expected %q, got %q", testData, buf)
	}
}

func TestProxy_HandleConnection_ContextCancelled(t *testing.T) {
	// Create a mock server that holds connections open
	blockListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blockListener.Close() }()

	go func() {
		for {
			conn, err := blockListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 1024)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	blockAddr := blockListener.Addr().String()

	dialer := &directDialer{addr: blockAddr}
	defer func() { _ = dialer.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  0,
		RemoteHost: "127.0.0.1",
		RemotePort: blockListener.Addr().(*net.TCPAddr).Port,
		Dialer:     dialer,
	}

	proxy, result, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("failed to start proxy: %v", xe)
	}

	// Connect to the proxy to create a handleConnection goroutine
	conn, err := net.DialTimeout("tcp", result.LocalAddress, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	_, _ = conn.Write([]byte("test"))

	// Cancel context to trigger the context.Done path in handleConnection
	cancel()

	// Stop should complete without hanging
	if err := proxy.Stop(); err != nil {
		t.Errorf("failed to stop proxy: %v", err)
	}
	_ = conn.Close()
}

// directDialer connects directly to a TCP address (no SSH tunnel).
type directDialer struct {
	addr string
}

func (d *directDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", d.addr)
}

func (d *directDialer) Close() error { return nil }

func TestProxy_HandleConnection_RemoteDisconnect(t *testing.T) {
	// Remote server that immediately closes accepted connection
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	go func() {
		for {
			conn, err := server.Accept()
			if err != nil {
				return
			}
			// Close remote side immediately
			_ = conn.Close()
		}
	}()

	dialer := &directDialer{addr: server.Addr().String()}
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  0,
		RemoteHost: "127.0.0.1",
		RemotePort: server.Addr().(*net.TCPAddr).Port,
		Dialer:     dialer,
	}

	proxy, result, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("failed to start proxy: %v", xe)
	}
	defer func() { _ = proxy.Stop() }()

	conn, err := net.DialTimeout("tcp", result.LocalAddress, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Because remote closed immediately, reading from local conn should return EOF quickly
	buf := make([]byte, 10)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected read error/EOF since remote closed")
	}
}

func TestProxy_HandleConnection_LocalDisconnect(t *testing.T) {
	// Remote server that holds connection open
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	remoteClosed := make(chan struct{})
	go func() {
		conn, err := server.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		// Wait for proxy to close remote connection when local client closes
		buf := make([]byte, 10)
		_, _ = conn.Read(buf)
		close(remoteClosed)
	}()

	dialer := &directDialer{addr: server.Addr().String()}
	defer func() { _ = dialer.Close() }()

	ctx := context.Background()
	opts := Options{
		LocalHost:  "127.0.0.1",
		LocalPort:  0,
		RemoteHost: "127.0.0.1",
		RemotePort: server.Addr().(*net.TCPAddr).Port,
		Dialer:     dialer,
	}

	proxy, result, xe := Start(ctx, opts)
	if xe != nil {
		t.Fatalf("failed to start proxy: %v", xe)
	}
	defer func() { _ = proxy.Stop() }()

	conn, err := net.DialTimeout("tcp", result.LocalAddress, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}

	// Close local connection immediately
	_ = conn.Close()

	select {
	case <-remoteClosed:
		// Success: remote connection unblocked and closed
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for remote connection to be closed after local client disconnect")
	}
}

type errDialer struct{}

func (d *errDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return nil, &net.OpError{Op: "dial", Err: &net.AddrError{Err: "simulated dial error", Addr: addr}}
}

func (d *errDialer) Close() error { return nil }

func TestProxy_HandleConnection_DialError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &Proxy{
		dialer: &errDialer{},
		ctx:    ctx,
		cancel: cancel,
	}

	localConn, peer := net.Pipe()
	defer func() { _ = peer.Close() }()

	done := make(chan struct{})
	p.wg.Add(1)
	go func() {
		defer close(done)
		p.handleConnection(localConn, "127.0.0.1:12345")
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConnection should return quickly when dialer returns error")
	}
}

type mockBackoffListener struct {
	attempts int
	maxErrs  int
	ch       chan net.Conn
	closed   chan struct{}
}

func (m *mockBackoffListener) Accept() (net.Conn, error) {
	m.attempts++
	if m.attempts <= m.maxErrs {
		return nil, &net.OpError{Op: "accept", Err: &net.AddrError{Err: "simulated accept error", Addr: "127.0.0.1"}}
	}
	select {
	case conn := <-m.ch:
		return conn, nil
	case <-m.closed:
		return nil, net.ErrClosed
	}
}

func (m *mockBackoffListener) Close() error {
	select {
	case <-m.closed:
	default:
		close(m.closed)
	}
	return nil
}

func (m *mockBackoffListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func TestProxy_AcceptConnections_BackoffAndRecover(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener := &mockBackoffListener{
		maxErrs: 2,
		ch:      make(chan net.Conn, 1),
		closed:  make(chan struct{}),
	}
	defer func() { _ = listener.Close() }()

	p := &Proxy{
		dialer:   &directDialer{addr: "127.0.0.1:0"},
		listener: listener,
		ctx:      ctx,
		cancel:   cancel,
	}

	localConn, peer := net.Pipe()
	defer func() { _ = peer.Close() }()
	listener.ch <- localConn

	p.wg.Add(1)
	go p.acceptConnections("127.0.0.1", 12345)

	// Wait for backoff to retry, accept the conn, and reset
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop accept loop
	cancel()
	_ = listener.Close()
	p.wg.Wait()
}

func TestProxy_AcceptConnections_ContextDoneDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	listener := &mockBackoffListener{
		maxErrs: 100, // Always error
		ch:      make(chan net.Conn, 1),
		closed:  make(chan struct{}),
	}
	defer func() { _ = listener.Close() }()

	p := &Proxy{
		dialer:   &nilConnDialer{},
		listener: listener,
		ctx:      ctx,
		cancel:   cancel,
	}

	p.wg.Add(1)
	go p.acceptConnections("127.0.0.1", 12345)

	// Cancel shortly after it hits the first error and enters time.After
	time.Sleep(2 * time.Millisecond)
	cancel()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("acceptConnections should terminate upon context cancellation during backoff")
	}
}



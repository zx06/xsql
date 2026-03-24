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

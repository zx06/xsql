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
				defer dialer.Close()
				tt.opts.Dialer = dialer
			}

			proxy, result, xe := Start(ctx, tt.opts)

			if tt.expectError {
				if xe == nil {
					t.Error("expected error but got none")
				}
				if proxy != nil {
					proxy.Stop()
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
			conn.Close()

			proxy.Stop()
		})
	}
}

func TestProxyStop(t *testing.T) {
	dialer := newMockSSHClient(t, "127.0.0.1:18090")
	defer dialer.Close()

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
	conn.Close()

	// Stop the proxy
	if err := proxy.Stop(); err != nil {
		t.Errorf("failed to stop proxy: %v", err)
	}

	// Verify proxy is no longer listening
	conn, err = net.DialTimeout("tcp", result.LocalAddress, 1*time.Second)
	if err == nil {
		conn.Close()
		t.Error("proxy should not be listening after stop")
	}
}

func TestProxyLocalAddress(t *testing.T) {
	dialer := newMockSSHClient(t, "127.0.0.1:18100")
	defer dialer.Close()

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
	defer proxy.Stop()

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
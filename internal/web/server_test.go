package web

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewServer creates and validates a new server
func TestNewServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewServer(listener, handler)

	if server == nil {
		t.Error("expected non-nil server")
	}
	if server.listener == nil {
		t.Error("expected non-nil listener")
	}
	if server.server == nil {
		t.Error("expected non-nil http.Server")
	}
	if server.server.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

// TestServerAddr validates the Addr method
func TestServerAddr(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	server := NewServer(listener, nil)
	addr := server.Addr()

	if addr == "" {
		t.Error("expected non-empty address")
	}

	// Address should be the listener's address
	expectedAddr := listener.Addr().String()
	if addr != expectedAddr {
		t.Errorf("expected addr %s, got %s", expectedAddr, addr)
	}
}

// TestServerAddr_NilServer tests Addr with nil server
func TestServerAddr_NilServer(t *testing.T) {
	var server *Server
	addr := server.Addr()

	if addr != "" {
		t.Errorf("expected empty address for nil server, got %s", addr)
	}
}

// TestServerAddr_NilListener tests Addr with nil listener
func TestServerAddr_NilListener(t *testing.T) {
	server := &Server{
		listener: nil,
		server:   &http.Server{},
	}
	addr := server.Addr()

	if addr != "" {
		t.Errorf("expected empty address for nil listener, got %s", addr)
	}
}

// TestServerShutdown validates graceful shutdown
func TestServerShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewServer(listener, handler)

	// Start server in a goroutine
	go server.Serve()
	defer listener.Close()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("unexpected error during shutdown: %v", err)
	}
}

// TestServerShutdown_NilServer tests shutdown with nil server
func TestServerShutdown_NilServer(t *testing.T) {
	var server *Server
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("expected nil error for nil server, got %v", err)
	}
}

// TestServerShutdown_NilHTTPServer tests shutdown with nil http.Server
func TestServerShutdown_NilHTTPServer(t *testing.T) {
	server := &Server{
		listener: nil,
		server:   nil,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("expected nil error for nil http.Server, got %v", err)
	}
}

// TestServerServe_NilServer tests Serve with nil server
func TestServerServe_NilServer(t *testing.T) {
	var server *Server
	err := server.Serve()

	if err != http.ErrServerClosed {
		t.Errorf("expected ErrServerClosed, got %v", err)
	}
}

// TestServerServe_NilHTTPServer tests Serve with nil http.Server
func TestServerServe_NilHTTPServer(t *testing.T) {
	server := &Server{
		listener: &net.TCPListener{},
		server:   nil,
	}
	err := server.Serve()

	if err != http.ErrServerClosed {
		t.Errorf("expected ErrServerClosed, got %v", err)
	}
}

// TestServerServe_NilListener tests Serve with nil listener
func TestServerServe_NilListener(t *testing.T) {
	server := &Server{
		listener: nil,
		server:   &http.Server{},
	}
	err := server.Serve()

	if err != http.ErrServerClosed {
		t.Errorf("expected ErrServerClosed, got %v", err)
	}
}

// TestDefaultAddrConstant validates the default address constant
func TestDefaultAddrConstant(t *testing.T) {
	expected := "127.0.0.1:8788"
	if DefaultAddr != expected {
		t.Errorf("expected DefaultAddr %s, got %s", expected, DefaultAddr)
	}
}

// TestServerWithTestListener validates server works with httptest listener
func TestServerWithTestListener(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	}))
	defer server.Close()

	// Verify the test server is working
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to get test server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %d", resp.StatusCode)
	}
}

// TestServerTimeouts validates that timeouts are properly configured
func TestServerTimeouts(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	server := NewServer(listener, nil)

	// Check that timeouts are set
	if server.server.ReadHeaderTimeout == 0 {
		t.Error("expected non-zero ReadHeaderTimeout")
	}
	if server.server.ReadTimeout == 0 {
		t.Error("expected non-zero ReadTimeout")
	}
	if server.server.WriteTimeout == 0 {
		t.Error("expected non-zero WriteTimeout")
	}
	if server.server.IdleTimeout == 0 {
		t.Error("expected non-zero IdleTimeout")
	}

	// Verify expected timeout values
	expectedRead := 15 * time.Second
	if server.server.ReadHeaderTimeout != expectedRead {
		t.Errorf("expected ReadHeaderTimeout %v, got %v", expectedRead, server.server.ReadHeaderTimeout)
	}

	expectedWrite := 30 * time.Second
	if server.server.WriteTimeout != expectedWrite {
		t.Errorf("expected WriteTimeout %v, got %v", expectedWrite, server.server.WriteTimeout)
	}
}

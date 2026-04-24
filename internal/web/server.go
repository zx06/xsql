package web

import (
	"context"
	"net"
	"net/http"
	"time"
)

const (
	// DefaultAddr is the default listen address for the web server.
	DefaultAddr = "127.0.0.1:8788"
	apiPrefix   = "/api/v1"
)

// Server wraps an HTTP server plus its listener.
type Server struct {
	listener net.Listener
	server   *http.Server
}

// NewServer creates a server for the provided listener and handler.
func NewServer(listener net.Listener, handler http.Handler) *Server {
	return &Server{
		listener: listener,
		server: &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 15 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

// Addr returns the effective listen address.
func (s *Server) Addr() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Serve starts serving HTTP requests.
func (s *Server) Serve() error {
	if s == nil || s.server == nil || s.listener == nil {
		return http.ErrServerClosed
	}
	return s.server.Serve(s.listener)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

package mysql

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
)

func TestDriver_Registered(t *testing.T) {
	drv, ok := db.Get("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}
	if drv == nil {
		t.Fatal("mysql driver is nil")
	}
}

func TestDriver_Open_InvalidDSN(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		DSN: "invalid:::dsn",
	}
	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestDriver_Open_ConnectionRefused(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     59999,
		User:     "test",
		Password: "test",
		Database: "test",
	}
	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
}

type mockDialer struct {
	called bool
}

func (m *mockDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	m.called = true
	return nil, net.ErrClosed
}

func TestDriver_Open_WithDialer(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dialer := &mockDialer{}
	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "test",
		Password: "test",
		Database: "test",
		Dialer:   dialer,
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected error from mock dialer")
	}
	if !dialer.called {
		t.Error("expected custom dialer to be called")
	}
}

func TestDriver_Open_WithDSN_Valid(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		DSN: "root:password@tcp(127.0.0.1:3306)/testdb?timeout=1s",
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error for invalid DSN")
	}
}

func TestDriver_Open_WithDSN_InvalidFormat(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		DSN: "invalid",
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected error for malformed DSN")
	}
}

func TestDriver_Open_NoDBName(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     59999,
		User:     "test",
		Password: "test",
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
}

func TestDriver_Open_WithParams(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     59999,
		User:     "test",
		Password: "test",
		Database: "test",
		Params: map[string]string{
			"charset":   "utf8mb4",
			"parseTime": "true",
		},
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
}

func TestDriver_Open_WithEmptyParams(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     59999,
		User:     "test",
		Password: "test",
		Database: "test",
		Params:   map[string]string{},
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
}

func TestDriver_Open_ContextCancelled(t *testing.T) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "test",
		Password: "test",
		Database: "test",
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected error for cancelled context")
	}
}

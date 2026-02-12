package pg

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
)

func TestDriver_Registered(t *testing.T) {
	drv, ok := db.Get("pg")
	if !ok {
		t.Fatal("pg driver not registered")
	}
	if drv == nil {
		t.Fatal("pg driver is nil")
	}
}

func TestDriver_Open_ConnectionRefused(t *testing.T) {
	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     59998,
		User:     "test",
		Password: "test",
		Database: "test",
	}
	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		opts     db.ConnOptions
		expected []string
	}{
		{
			name: "full options",
			opts: db.ConnOptions{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "mydb",
			},
			expected: []string{"host=localhost", "port=5432", "user=user", "password=pass", "dbname=mydb"},
		},
		{
			name: "partial options",
			opts: db.ConnOptions{
				Host: "localhost",
				User: "user",
			},
			expected: []string{"host=localhost", "user=user"},
		},
		{
			name:     "empty options",
			opts:     db.ConnOptions{},
			expected: []string{},
		},
		{
			name: "with port zero",
			opts: db.ConnOptions{
				Host:     "localhost",
				Port:     0,
				User:     "user",
				Password: "pass",
				Database: "mydb",
			},
			expected: []string{"host=localhost", "user=user", "password=pass", "dbname=mydb"},
		},
		{
			name: "with params",
			opts: db.ConnOptions{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "mydb",
				Params: map[string]string{
					"sslmode":        "disable",
					"pool_max_conns": "10",
				},
			},
			expected: []string{"sslmode=disable", "pool_max_conns=10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildDSN(tt.opts)
			for _, frag := range tt.expected {
				if len(frag) > 0 && !containsSubstring(dsn, frag) {
					t.Errorf("DSN %q should contain %q", dsn, frag)
				}
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockDialer struct {
	called bool
}

func (m *mockDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	m.called = true
	return nil, net.ErrClosed
}

func TestDriver_Open_WithDialer(t *testing.T) {
	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dialer := &mockDialer{}
	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     5432,
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

func TestDriver_Open_WithDSN(t *testing.T) {
	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		DSN: "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable",
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
}

func TestDriver_Open_WithDSN_Invalid(t *testing.T) {
	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	opts := db.ConnOptions{
		DSN: "invalid://",
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestDriver_Open_WithDSN_AndDialer(t *testing.T) {
	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dialer := &mockDialer{}
	opts := db.ConnOptions{
		DSN:    "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable",
		Dialer: dialer,
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
	if !dialer.called {
		t.Error("expected custom dialer to be called when DSN and Dialer both provided")
	}
}

func TestDriver_Open_ContextCancelled(t *testing.T) {
	drv, _ := db.Get("pg")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "test",
		Password: "test",
		Database: "test",
	}

	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected error for cancelled context")
	}
}

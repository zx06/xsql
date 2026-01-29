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
		Port:     59998, // 不太可能有服务监听的端口
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
		expected []string // 应包含的片段
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

// mockDialer 用于测试自定义 dialer
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
	// 应该失败，但 dialer 应该被调用
	if xe == nil {
		t.Fatal("expected error from mock dialer")
	}
	if !dialer.called {
		t.Error("expected custom dialer to be called")
	}
}

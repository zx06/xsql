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

	// 使用无效的 DSN 格式
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

	// 使用不存在的地址
	opts := db.ConnOptions{
		Host:     "127.0.0.1",
		Port:     59999, // 不太可能有服务监听的端口
		User:     "test",
		Password: "test",
		Database: "test",
	}
	_, xe := drv.Open(ctx, opts)
	if xe == nil {
		t.Fatal("expected connection error")
	}
}

// mockDialer 用于测试自定义 dialer 注册
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
	// 应该失败，但 dialer 应该被调用
	if xe == nil {
		t.Fatal("expected error from mock dialer")
	}
	if !dialer.called {
		t.Error("expected custom dialer to be called")
	}
}

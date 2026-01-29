package db

import (
	"context"
	"database/sql"
	"sync"

	"github.com/zx06/xsql/internal/errors"
)

// Driver 是数据库驱动的最小抽象。
type Driver interface {
	// Open 返回 *sql.DB；由具体 driver 实现连接参数解析。
	Open(ctx context.Context, opts ConnOptions) (*sql.DB, *errors.XError)
}

// ConnOptions 是通用连接参数（由 config/CLI/ENV 合并而来）。
type ConnOptions struct {
	DSN      string // 原生 DSN（优先级最高）
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Params   map[string]string // 额外参数
}

var (
	mu      sync.RWMutex
	drivers = map[string]Driver{}
)

func Register(name string, d Driver) {
	mu.Lock()
	defer mu.Unlock()
	if name == "" {
		panic("db.Register: empty name")
	}
	if d == nil {
		panic("db.Register: nil driver")
	}
	if _, exists := drivers[name]; exists {
		panic("db.Register: duplicate driver: " + name)
	}
	drivers[name] = d
}

func Get(name string) (Driver, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := drivers[name]
	return d, ok
}

func RegisteredNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(drivers))
	for k := range drivers {
		out = append(out, k)
	}
	return out
}

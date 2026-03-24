// Package db provides the database driver registry and query execution engine.
package db

import (
	"context"
	"database/sql"
	"net"
	"sync"

	"github.com/zx06/xsql/internal/errors"
)

// Dialer is used for custom network connections (e.g. SSH tunnel).
type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// Driver is the minimal abstraction for a database driver.
type Driver interface {
	// Open returns a *sql.DB; connection parameter parsing is handled by each driver.
	Open(ctx context.Context, opts ConnOptions) (*sql.DB, *errors.XError)
}

// ConnOptions holds common connection parameters (merged from config/CLI/ENV).
type ConnOptions struct {
	DSN      string // Raw DSN (highest priority)
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Params   map[string]string // Extra parameters
	Dialer   Dialer            // Custom dialer (e.g. SSH tunnel)
	// RegisterCloseHook allows drivers to register cleanup callbacks that should run
	// when the owning connection is closed or setup fails.
	RegisterCloseHook func(fn func())
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

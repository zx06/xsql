// Package mysql implements the MySQL database driver.
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/go-sql-driver/mysql"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

var (
	dialerCounter uint64
	dialers       sync.Map
)

func init() {
	db.Register("mysql", &Driver{})
}

func registerDialContext(dialer func(context.Context, string, string) (net.Conn, error)) string {
	dialerNum := atomic.AddUint64(&dialerCounter, 1)
	dialName := fmt.Sprintf("xsql_ssh_tunnel_%d", dialerNum)

	mysql.RegisterDialContext(dialName, func(ctx context.Context, addr string) (net.Conn, error) {
		d, ok := dialers.Load(dialName)
		if !ok {
			return nil, fmt.Errorf("dialer not found: %s", dialName)
		}
		fn, ok := d.(func(context.Context, string, string) (net.Conn, error))
		if !ok || fn == nil {
			return nil, fmt.Errorf("invalid dialer for: %s", dialName)
		}
		return fn(ctx, "tcp", addr)
	})

	dialers.Store(dialName, dialer)
	return dialName
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
	cfg := mysql.NewConfig()
	var dialName string

	if opts.DSN != "" {
		parsed, err := mysql.ParseDSN(opts.DSN)
		if err != nil {
			return nil, errors.Wrap(errors.CodeCfgInvalid, "invalid mysql dsn", nil, err)
		}
		cfg = parsed
	} else {
		cfg.User = opts.User
		cfg.Passwd = opts.Password
		cfg.Net = "tcp"
		cfg.Addr = fmt.Sprintf("%s:%d", opts.Host, opts.Port)
		cfg.DBName = opts.Database
		cfg.ParseTime = true
		if cfg.Params == nil {
			cfg.Params = make(map[string]string)
		}
		for k, v := range opts.Params {
			cfg.Params[k] = v
		}
	}

	if opts.Dialer != nil {
		dialName = registerDialContext(opts.Dialer.DialContext)
		cfg.Net = dialName
		if opts.RegisterCloseHook != nil {
			opts.RegisterCloseHook(func() {
				dialers.Delete(dialName)
				mysql.DeregisterDialContext(dialName)
			})
		}
	}

	dsn := cfg.FormatDSN()
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to open mysql connection", nil, err)
	}
	if err := conn.PingContext(ctx); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("failed to close mysql connection: %v", closeErr)
		}
		if dialName != "" {
			dialers.Delete(dialName)
		}
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to ping mysql", nil, err)
	}
	return conn, nil
}

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"sync/atomic"

	"github.com/go-sql-driver/mysql"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

var currentDialer atomic.Value

func init() {
	db.Register("mysql", &Driver{})
	mysql.RegisterDialContext("xsql_ssh_tunnel", func(ctx context.Context, addr string) (net.Conn, error) {
		dialer, ok := currentDialer.Load().(func(context.Context, string, string) (net.Conn, error))
		if !ok || dialer == nil {
			return nil, fmt.Errorf("no dialer configured")
		}
		return dialer(ctx, "tcp", addr)
	})
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
	cfg := mysql.NewConfig()

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
		currentDialer.Store(opts.Dialer.DialContext)
		cfg.Net = "xsql_ssh_tunnel"
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
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to ping mysql", nil, err)
	}
	return conn, nil
}

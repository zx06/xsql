package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"sync"

	"github.com/go-sql-driver/mysql"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

var (
	dialerOnce   sync.Once
	dialerName   string
	dialerCalled bool
)

func init() {
	db.Register("mysql", &Driver{})
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
		dialerOnce.Do(func() {
			dialerName = "xsql_ssh_tunnel"
			mysql.RegisterDialContext(dialerName, func(ctx context.Context, addr string) (net.Conn, error) {
				return opts.Dialer.DialContext(ctx, "tcp", addr)
			})
			dialerCalled = true
		})
		if !dialerCalled {
			mysql.RegisterDialContext(dialerName, func(ctx context.Context, addr string) (net.Conn, error) {
				return opts.Dialer.DialContext(ctx, "tcp", addr)
			})
		}
		cfg.Net = dialerName
	}

	dsn := cfg.FormatDSN()
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to open mysql connection", nil, err)
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to ping mysql", nil, err)
	}
	return conn, nil
}

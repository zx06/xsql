package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/go-sql-driver/mysql"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

var dialerCounter uint64

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

	// 注册自定义 dialer（用于 SSH tunnel）
	if opts.Dialer != nil {
		netName := fmt.Sprintf("xsql_ssh_%d", atomic.AddUint64(&dialerCounter, 1))
		mysql.RegisterDialContext(netName, func(ctx context.Context, addr string) (net.Conn, error) {
			return opts.Dialer.DialContext(ctx, "tcp", addr)
		})
		cfg.Net = netName
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

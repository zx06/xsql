package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

func init() {
	db.Register("mysql", &Driver{})
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
	dsn := opts.DSN
	if dsn == "" {
		cfg := mysql.NewConfig()
		cfg.User = opts.User
		cfg.Passwd = opts.Password
		cfg.Net = "tcp"
		cfg.Addr = fmt.Sprintf("%s:%d", opts.Host, opts.Port)
		cfg.DBName = opts.Database
		cfg.ParseTime = true
		for k, v := range opts.Params {
			cfg.Params[k] = v
		}
		dsn = cfg.FormatDSN()
	}
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

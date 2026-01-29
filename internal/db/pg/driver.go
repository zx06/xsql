package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

func init() {
	db.Register("pg", &Driver{})
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
	dsn := opts.DSN
	if dsn == "" {
		dsn = buildDSN(opts)
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to open pg connection", nil, err)
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to ping pg", nil, err)
	}
	return conn, nil
}

func buildDSN(opts db.ConnOptions) string {
	parts := []string{}
	if opts.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", opts.Host))
	}
	if opts.Port != 0 {
		parts = append(parts, fmt.Sprintf("port=%d", opts.Port))
	}
	if opts.User != "" {
		parts = append(parts, fmt.Sprintf("user=%s", opts.User))
	}
	if opts.Password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", opts.Password))
	}
	if opts.Database != "" {
		parts = append(parts, fmt.Sprintf("dbname=%s", opts.Database))
	}
	for k, v := range opts.Params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, " ")
}

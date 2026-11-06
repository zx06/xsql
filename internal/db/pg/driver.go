// Package pg implements the PostgreSQL database driver.
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

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

	// Use pgx custom dialer
	if opts.Dialer != nil {
		config, err := pgx.ParseConfig(dsn)
		if err != nil {
			return nil, errors.Wrap(errors.CodeCfgInvalid, "invalid pg dsn", nil, err)
		}
		config.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return opts.Dialer.DialContext(ctx, network, addr)
		}
		conn := stdlib.OpenDB(*config)
		if err := conn.PingContext(ctx); err != nil {
			if closeErr := conn.Close(); closeErr != nil {
				log.Printf("failed to close pg connection: %v", closeErr)
			}
			return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to ping pg", map[string]any{"dialect": "pg"}, err)
		}
		return conn, nil
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to open pg connection", map[string]any{"dialect": "pg"}, err)
	}
	if err := conn.PingContext(ctx); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("failed to close pg connection: %v", closeErr)
		}
		return nil, errors.Wrap(errors.CodeDBConnectFailed, "failed to ping pg", map[string]any{"dialect": "pg"}, err)
	}
	return conn, nil
}

func buildDSN(opts db.ConnOptions) string {
	parts := []string{}
	if opts.Host != "" {
		parts = append(parts, formatPGParam("host", opts.Host))
	}
	if opts.Port != 0 {
		parts = append(parts, fmt.Sprintf("port=%d", opts.Port))
	}
	if opts.User != "" {
		parts = append(parts, formatPGParam("user", opts.User))
	}
	if opts.Password != "" {
		parts = append(parts, formatPGParam("password", opts.Password))
	}
	if opts.Database != "" {
		parts = append(parts, formatPGParam("dbname", opts.Database))
	}
	for k, v := range opts.Params {
		parts = append(parts, formatPGParam(k, v))
	}
	return strings.Join(parts, " ")
}

func formatPGParam(key, val string) string {
	if strings.ContainsAny(val, " \t\n\r'\\=") {
		escaped := strings.ReplaceAll(val, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `'`, `\'`)
		return fmt.Sprintf("%s='%s'", key, escaped)
	}
	return fmt.Sprintf("%s=%s", key, val)
}

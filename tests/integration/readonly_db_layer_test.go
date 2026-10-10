//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
)

func TestReadOnlyTx_DatabaseLayer(t *testing.T) {
	cases := []struct {
		name             string
		driver           string
		dsnEnv           string
		dbType           string
		sql              string
		bypassStatic     bool
		wantErr          bool
		wantCode         errors.Code
		wantMinRows      int
	}{
		{
			name:        "mysql_select_ok",
			driver:      "mysql",
			dsnEnv:      "XSQL_TEST_MYSQL_DSN",
			dbType:      "mysql",
			sql:         "SELECT 1 AS n",
			wantMinRows: 1,
		},
		{
			name:         "mysql_insert_blocked",
			driver:       "mysql",
			dsnEnv:       "XSQL_TEST_MYSQL_DSN",
			dbType:       "mysql",
			sql:          "INSERT INTO nonexistent_table_xsql_ro VALUES (1)",
			bypassStatic: true,
			wantErr:      true,
			wantCode:     errors.CodeDBExecFailed,
		},
		{
			name:         "pg_insert_blocked",
			driver:       "pg",
			dsnEnv:       "XSQL_TEST_PG_DSN",
			dbType:       "pg",
			sql:          "INSERT INTO nonexistent_table_xsql_ro VALUES (1)",
			bypassStatic: true,
			wantErr:      true,
			wantCode:     errors.CodeDBExecFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.dsnEnv)
			if dsn == "" {
				t.Skipf("%s not set", tc.dsnEnv)
			}

			drv, ok := db.Get(tc.driver)
			if !ok {
				t.Fatalf("%s driver not registered", tc.driver)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
			if xe != nil {
				t.Fatalf("failed to open: %v", xe)
			}
			defer conn.Close()

			result, xe := db.Query(ctx, conn, tc.sql, db.QueryOptions{
				DBType:               tc.dbType,
				BypassStaticReadOnly: tc.bypassStatic,
			})
			if tc.wantErr {
				if xe == nil {
					t.Fatal("expected error")
				}
				if tc.wantCode != "" && xe.Code != tc.wantCode {
					t.Fatalf("expected code %s, got %s", tc.wantCode, xe.Code)
				}
				return
			}
			if xe != nil {
				t.Fatalf("unexpected error: %v", xe)
			}
			if len(result.Rows) < tc.wantMinRows {
				t.Fatalf("expected at least %d rows, got %d", tc.wantMinRows, len(result.Rows))
			}
		})
	}
}

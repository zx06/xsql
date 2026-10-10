//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql" // register mysql driver
	_ "github.com/zx06/xsql/internal/db/pg"    // register pg driver
)

func TestListTables_IncludeSystem(t *testing.T) {
	cases := []struct {
		name       string
		envDSN     string
		driver     string
		pattern    string
		wantSchema string
		wantTable  string
	}{
		{
			name:       "mysql_db",
			envDSN:     "XSQL_TEST_MYSQL_DSN",
			driver:     "mysql",
			pattern:    "db",
			wantSchema: "mysql",
			wantTable:  "db",
		},
		{
			name:       "pg_pg_namespace",
			envDSN:     "XSQL_TEST_PG_DSN",
			driver:     "pg",
			pattern:    "pg_namespace",
			wantSchema: "pg_catalog",
			wantTable:  "pg_namespace",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.envDSN)
			if dsn == "" {
				t.Skip(tc.envDSN + " not set")
			}
			drv, _ := db.Get(tc.driver)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
			if xe != nil {
				t.Fatalf("open: %v", xe)
			}
			defer conn.Close()

			list, xe := db.ListTables(ctx, tc.driver, conn, db.SchemaOptions{
				TablePattern:  tc.pattern,
				IncludeSystem: true,
			})
			if xe != nil {
				t.Fatalf("ListTables: %v", xe)
			}
			for _, tbl := range list.Tables {
				if tbl.Schema == tc.wantSchema && tbl.Name == tc.wantTable {
					return
				}
			}
			t.Fatalf("expected %s.%s, got %d tables", tc.wantSchema, tc.wantTable, len(list.Tables))
		})
	}
}

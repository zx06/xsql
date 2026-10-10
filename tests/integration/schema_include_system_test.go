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
			assertIncludeSystemTable(t, tc.envDSN, tc.driver, tc.pattern, tc.wantSchema, tc.wantTable)
		})
	}
}

func assertIncludeSystemTable(t *testing.T, envVar, driver, pattern, wantSchema, wantTable string) {
	t.Helper()
	dsn := os.Getenv(envVar)
	if dsn == "" {
		t.Skip(envVar + " not set")
	}
	drv, _ := db.Get(driver)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("open: %v", xe)
	}
	defer conn.Close()

	list, xe := db.ListTables(ctx, driver, conn, db.SchemaOptions{
		TablePattern:  pattern,
		IncludeSystem: true,
	})
	if xe != nil {
		t.Fatalf("ListTables: %v", xe)
	}
	for _, tbl := range list.Tables {
		if tbl.Schema == wantSchema && tbl.Name == wantTable {
			return
		}
	}
	t.Fatalf("expected %s.%s, got %d tables", wantSchema, wantTable, len(list.Tables))
}

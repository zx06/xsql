//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
)

func TestMySQL_ListTables_IncludeSystemDbTable(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("open: %v", xe)
	}
	defer conn.Close()

	list, xe := db.ListTables(ctx, "mysql", conn, db.SchemaOptions{
		TablePattern:  "db",
		IncludeSystem: true,
	})
	if xe != nil {
		t.Fatalf("ListTables: %v", xe)
	}
	found := false
	for _, tbl := range list.Tables {
		if tbl.Schema == "mysql" && tbl.Name == "db" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mysql.db with include_system, got %d tables", len(list.Tables))
	}
}

func TestPg_ListTables_IncludeSystemPgNamespace(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}
	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("open: %v", xe)
	}
	defer conn.Close()

	list, xe := db.ListTables(ctx, "pg", conn, db.SchemaOptions{
		TablePattern:  "pg_namespace",
		IncludeSystem: true,
	})
	if xe != nil {
		t.Fatalf("ListTables: %v", xe)
	}
	found := false
	for _, tbl := range list.Tables {
		if tbl.Schema == "pg_catalog" && tbl.Name == "pg_namespace" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pg_catalog.pg_namespace, got %d tables", len(list.Tables))
	}
}

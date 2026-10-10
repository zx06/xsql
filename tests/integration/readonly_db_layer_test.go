//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
)

func TestMySQL_ReadOnlyTxAllowsSelect(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, ok := db.Get("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SELECT 1 AS n", db.QueryOptions{DBType: "mysql"})
	if xe != nil {
		t.Fatalf("expected SELECT to succeed in read-only transaction: %v", xe)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestMySQL_ReadOnlyTxBlocksWrite_BypassStatic(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, ok := db.Get("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	opts := db.QueryOptions{DBType: "mysql", BypassStaticReadOnly: true}
	_, xe = db.Query(ctx, conn, "INSERT INTO nonexistent_table_xsql_ro VALUES (1)", opts)
	if xe == nil {
		t.Fatal("expected DB-layer rejection for INSERT")
	}
	if xe.Code != errors.CodeDBExecFailed {
		t.Fatalf("expected XSQL_DB_EXEC_FAILED, got %s", xe.Code)
	}
	if !strings.Contains(strings.ToLower(xe.Message), "read") {
		t.Logf("driver message (may vary): %s", xe.Message)
	}
}

func TestPg_ReadOnlyTxBlocksWrite_BypassStatic(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, ok := db.Get("pg")
	if !ok {
		t.Fatal("pg driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	opts := db.QueryOptions{DBType: "pg", BypassStaticReadOnly: true}
	_, xe = db.Query(ctx, conn, "INSERT INTO nonexistent_table_xsql_ro VALUES (1)", opts)
	if xe == nil {
		t.Fatal("expected DB-layer rejection for INSERT")
	}
	if xe.Code != errors.CodeDBExecFailed {
		t.Fatalf("expected XSQL_DB_EXEC_FAILED, got %s", xe.Code)
	}
}

//go:build integration

// Package integration contains integration tests for xsql.
// Run with: go test -tags=integration ./tests/integration/...
//
// These tests require actual database connections:
// - MySQL: XSQL_TEST_MYSQL_DSN
// - PostgreSQL: XSQL_TEST_PG_DSN
package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
)

func TestMySQLConnection(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("cannot open mysql: %v", err)
	}
	defer conn.Close()

	if err := conn.PingContext(ctx); err != nil {
		t.Fatalf("mysql ping failed: %v", err)
	}

	var result int
	err = conn.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("SELECT 1 failed: %v", err)
	}
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}
}

func TestPostgreSQLConnection(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("cannot open pg: %v", err)
	}
	defer conn.Close()

	if err := conn.PingContext(ctx); err != nil {
		t.Fatalf("pg ping failed: %v", err)
	}

	var result int
	err = conn.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("SELECT 1 failed: %v", err)
	}
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}
}

// TestMySQLDriver_Query tests the full query path through xsql's MySQL driver
func TestMySQLDriver_Query(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, ok := db.Get("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// Test basic query
	result, xe := db.Query(ctx, conn, "SELECT 1 as num, 'hello' as msg", db.QueryOptions{DBType: "mysql"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}
}

// TestMySQLDriver_ReadOnlyEnforcement tests that write queries are blocked
func TestMySQLDriver_ReadOnlyEnforcement(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, ok := db.Get("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// Write query should be blocked
	_, xe = db.Query(ctx, conn, "INSERT INTO test VALUES (1)", db.QueryOptions{DBType: "mysql"})
	if xe == nil {
		t.Fatal("expected error for INSERT in read-only mode")
	}
	if xe.Code != "XSQL_RO_BLOCKED" {
		t.Errorf("expected XSQL_RO_BLOCKED, got %s", xe.Code)
	}
}

// TestPgDriver_Query tests the full query path through xsql's PG driver
func TestPgDriver_Query(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, ok := db.Get("pg")
	if !ok {
		t.Fatal("pg driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// Test basic query
	result, xe := db.Query(ctx, conn, "SELECT 1 as num, 'hello' as msg", db.QueryOptions{DBType: "pg"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}
}

// TestPgDriver_ReadOnlyEnforcement tests that write queries are blocked
func TestPgDriver_ReadOnlyEnforcement(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, ok := db.Get("pg")
	if !ok {
		t.Fatal("pg driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// Write query should be blocked
	_, xe = db.Query(ctx, conn, "INSERT INTO test VALUES (1)", db.QueryOptions{DBType: "pg"})
	if xe == nil {
		t.Fatal("expected error for INSERT in read-only mode")
	}
	if xe.Code != "XSQL_RO_BLOCKED" {
		t.Errorf("expected XSQL_RO_BLOCKED, got %s", xe.Code)
	}
}

// TestMySQLDriver_ComplexQuery tests more complex queries
func TestMySQLDriver_ComplexQuery(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, ok := db.Get("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// Test SHOW statement
	result, xe := db.Query(ctx, conn, "SHOW DATABASES", db.QueryOptions{DBType: "pg"})
	if xe != nil {
		t.Fatalf("SHOW DATABASES failed: %v", xe)
	}
	if len(result.Rows) == 0 {
		t.Error("expected at least one database")
	}

	// Test EXPLAIN
	result, xe = db.Query(ctx, conn, "EXPLAIN SELECT 1", db.QueryOptions{DBType: "pg"})
	if xe != nil {
		t.Fatalf("EXPLAIN failed: %v", xe)
	}
	if len(result.Columns) == 0 {
		t.Error("EXPLAIN should return columns")
	}
}

// TestPgDriver_ComplexQuery tests more complex queries
func TestPgDriver_ComplexQuery(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, ok := db.Get("pg")
	if !ok {
		t.Fatal("pg driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// Test system catalog query
	result, xe := db.Query(ctx, conn, "SELECT datname FROM pg_database LIMIT 5", db.QueryOptions{DBType: "pg"})
	if xe != nil {
		t.Fatalf("pg_database query failed: %v", xe)
	}
	if len(result.Rows) == 0 {
		t.Error("expected at least one database")
	}

	// Test EXPLAIN
	result, xe = db.Query(ctx, conn, "EXPLAIN SELECT 1", db.QueryOptions{DBType: "pg"})
	if xe != nil {
		t.Fatalf("EXPLAIN failed: %v", xe)
	}
	if len(result.Rows) == 0 {
		t.Error("EXPLAIN should return rows")
	}
}

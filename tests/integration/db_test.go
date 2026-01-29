//go:build integration

// Package integration contains integration tests for xsql.
// Run with: go test -tags=integration ./tests/integration/...
//
// These tests require actual database connections:
// - MySQL: XSQL_TEST_MYSQL_DSN or localhost:3306
// - PostgreSQL: XSQL_TEST_PG_DSN or localhost:5432
package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMySQLConnection(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("cannot open mysql: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("mysql not available: %v", err)
	}

	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
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
		dsn = "postgres://postgres:@127.0.0.1:5432/?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("cannot open pg: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("pg not available: %v", err)
	}

	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("SELECT 1 failed: %v", err)
	}
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}
}

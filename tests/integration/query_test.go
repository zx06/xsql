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

// ============== MySQL Query Tests ==============

func TestMySQL_Query_SelectBasic(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// Use simple NULL without alias for MySQL 8.0 compatibility
	result, xe := db.Query(ctx, conn, "SELECT 1 as num, 'hello' as msg", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	// Check columns
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
	if result.Columns[0] != "num" || result.Columns[1] != "msg" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}

	// Check rows
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	// Check values
	row := result.Rows[0]
	if row["msg"] != "hello" {
		t.Errorf("msg=%v, want 'hello'", row["msg"])
	}
}

func TestMySQL_Query_MultipleRows(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SELECT 1 as n UNION SELECT 2 UNION SELECT 3", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}
}

func TestMySQL_Query_EmptyResult(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SELECT 1 as n FROM (SELECT 1) t WHERE 1=0", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
	// Columns should still be present
	if len(result.Columns) != 1 || result.Columns[0] != "n" {
		t.Errorf("columns should be present even with no rows: %v", result.Columns)
	}
}

func TestMySQL_Query_ShowDatabases(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SHOW DATABASES", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe != nil {
		t.Fatalf("SHOW DATABASES failed: %v", xe)
	}

	if len(result.Rows) == 0 {
		t.Error("expected at least one database")
	}
}

func TestMySQL_Query_Explain(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "EXPLAIN SELECT 1", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe != nil {
		t.Fatalf("EXPLAIN failed: %v", xe)
	}

	if len(result.Columns) == 0 {
		t.Error("EXPLAIN should return columns")
	}
}

func TestMySQL_Query_ReadOnlyBlocked(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// INSERT should be blocked by read-only check (before hitting DB)
	_, xe = db.Query(ctx, conn, "INSERT INTO nonexistent VALUES (1)", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe == nil {
		t.Fatal("expected error for INSERT in read-only mode")
	}
	if xe.Code != "XSQL_RO_BLOCKED" {
		t.Errorf("expected XSQL_RO_BLOCKED, got %s", xe.Code)
	}

	// UPDATE should be blocked
	_, xe = db.Query(ctx, conn, "UPDATE nonexistent SET x=1", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe == nil || xe.Code != "XSQL_RO_BLOCKED" {
		t.Error("UPDATE should be blocked")
	}

	// DELETE should be blocked
	_, xe = db.Query(ctx, conn, "DELETE FROM nonexistent", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe == nil || xe.Code != "XSQL_RO_BLOCKED" {
		t.Error("DELETE should be blocked")
	}

	// DROP should be blocked
	_, xe = db.Query(ctx, conn, "DROP TABLE nonexistent", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe == nil || xe.Code != "XSQL_RO_BLOCKED" {
		t.Error("DROP should be blocked")
	}

	// CREATE should be blocked
	_, xe = db.Query(ctx, conn, "CREATE TABLE test (id INT)", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe == nil || xe.Code != "XSQL_RO_BLOCKED" {
		t.Error("CREATE should be blocked")
	}
}

func TestMySQL_Query_InvalidSQL(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	_, xe = db.Query(ctx, conn, "SELECT * FROM definitely_nonexistent_table_12345", db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe == nil {
		t.Fatal("expected error for invalid SQL")
	}
	if xe.Code != "XSQL_DB_EXEC_FAILED" {
		t.Errorf("expected XSQL_DB_EXEC_FAILED, got %s", xe.Code)
	}
}

func TestMySQL_Query_DataTypes(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, `
		SELECT 
			42 as int_val,
			3.14 as float_val,
			'text' as str_val,
			TRUE as bool_val,
			NOW() as time_val
	`, db.QueryOptions{ReadOnly: true, DBType: "mysql"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	row := result.Rows[0]
	if row["str_val"] != "text" {
		t.Errorf("str_val=%v, want 'text'", row["str_val"])
	}
}

// ============== PostgreSQL Query Tests ==============

func TestPg_Query_SelectBasic(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SELECT 1 as num, 'hello' as msg, NULL as empty", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	// Check columns
	if len(result.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(result.Columns))
	}

	// Check values
	row := result.Rows[0]
	if row["msg"] != "hello" {
		t.Errorf("msg=%v, want 'hello'", row["msg"])
	}
	if row["empty"] != nil {
		t.Errorf("empty=%v, want nil", row["empty"])
	}
}

func TestPg_Query_MultipleRows(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SELECT generate_series(1,5) as n", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Rows) != 5 {
		t.Errorf("expected 5 rows, got %d", len(result.Rows))
	}
}

func TestPg_Query_EmptyResult(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SELECT 1 as n WHERE 1=0", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
	if len(result.Columns) != 1 {
		t.Errorf("columns should be present even with no rows")
	}
}

func TestPg_Query_SystemCatalog(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "SELECT datname FROM pg_database LIMIT 5", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe != nil {
		t.Fatalf("pg_database query failed: %v", xe)
	}

	if len(result.Rows) == 0 {
		t.Error("expected at least one database")
	}
}

func TestPg_Query_Explain(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, "EXPLAIN SELECT 1", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe != nil {
		t.Fatalf("EXPLAIN failed: %v", xe)
	}

	if len(result.Rows) == 0 {
		t.Error("EXPLAIN should return rows")
	}
}

func TestPg_Query_ReadOnlyBlocked(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// INSERT should be blocked
	_, xe = db.Query(ctx, conn, "INSERT INTO nonexistent VALUES (1)", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe == nil || xe.Code != "XSQL_RO_BLOCKED" {
		t.Error("INSERT should be blocked")
	}

	// UPDATE should be blocked
	_, xe = db.Query(ctx, conn, "UPDATE nonexistent SET x=1", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe == nil || xe.Code != "XSQL_RO_BLOCKED" {
		t.Error("UPDATE should be blocked")
	}

	// DELETE should be blocked
	_, xe = db.Query(ctx, conn, "DELETE FROM nonexistent", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe == nil || xe.Code != "XSQL_RO_BLOCKED" {
		t.Error("DELETE should be blocked")
	}
}

func TestPg_Query_InvalidSQL(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	_, xe = db.Query(ctx, conn, "SELECT * FROM definitely_nonexistent_table_12345", db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe == nil {
		t.Fatal("expected error for invalid SQL")
	}
	if xe.Code != "XSQL_DB_EXEC_FAILED" {
		t.Errorf("expected XSQL_DB_EXEC_FAILED, got %s", xe.Code)
	}
}

func TestPg_Query_DataTypes(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	result, xe := db.Query(ctx, conn, `
		SELECT 
			42 as int_val,
			3.14::float as float_val,
			'text' as str_val,
			TRUE as bool_val,
			NOW() as time_val,
			'{"key": "value"}'::json as json_val
	`, db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}

	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	row := result.Rows[0]
	if row["str_val"] != "text" {
		t.Errorf("str_val=%v, want 'text'", row["str_val"])
	}
	if row["bool_val"] != true {
		t.Errorf("bool_val=%v, want true", row["bool_val"])
	}
}

func TestPg_Query_CTE(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, _ := db.Get("pg")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open: %v", xe)
	}
	defer conn.Close()

	// CTE (WITH clause) should work in read-only mode
	result, xe := db.Query(ctx, conn, `
		WITH numbers AS (
			SELECT generate_series(1, 3) as n
		)
		SELECT n * 2 as doubled FROM numbers
	`, db.QueryOptions{ReadOnly: true, DBType: "pg"})
	if xe != nil {
		t.Fatalf("CTE query failed: %v", xe)
	}

	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}
}

//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
)

func TestSchemaDump_MySQL_RealDB(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	drv, ok := db.Get("mysql")
	if !ok {
		t.Fatal("mysql driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open mysql: %v", xe)
	}
	defer conn.Close()

	suffix := time.Now().UnixNano()
	prefix := fmt.Sprintf("xsql_schema_%d", suffix)
	usersTable := prefix + "_users"
	ordersTable := prefix + "_orders"

	// 清理旧表
	_, _ = conn.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", ordersTable))
	_, _ = conn.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", usersTable))

	// 创建表结构
	_, err := conn.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id BIGINT PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			tenant_id BIGINT NOT NULL,
			created_at DATETIME NULL,
			INDEX idx_email (email)
		) ENGINE=InnoDB
	`, usersTable))
	if err != nil {
		t.Fatalf("create users table failed: %v", err)
	}

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id BIGINT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			CONSTRAINT fk_%s_user FOREIGN KEY (user_id) REFERENCES %s(id)
		) ENGINE=InnoDB
	`, ordersTable, ordersTable, usersTable))
	if err != nil {
		t.Fatalf("create orders table failed: %v", err)
	}

	t.Cleanup(func() {
		_, _ = conn.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", ordersTable))
		_, _ = conn.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", usersTable))
	})

	info, xe := db.DumpSchema(ctx, "mysql", conn, db.SchemaOptions{
		TablePattern: prefix + "*",
	})
	if xe != nil {
		t.Fatalf("DumpSchema error: %v", xe)
	}
	if info.Database == "" {
		t.Fatalf("database name is empty")
	}

	users := findTable(info.Tables, usersTable)
	orders := findTable(info.Tables, ordersTable)
	if users == nil || orders == nil {
		t.Fatalf("missing tables in schema dump: users=%v orders=%v", users != nil, orders != nil)
	}

	if users.Schema == "" {
		t.Fatalf("users schema is empty")
	}
	if len(users.Columns) == 0 {
		t.Fatalf("users columns should not be empty")
	}

	if !hasColumn(users, "id", true) {
		t.Fatalf("users table missing primary key column 'id'")
	}
	if !hasIndex(users, "PRIMARY") {
		t.Fatalf("users table missing PRIMARY index")
	}
	if !hasIndex(users, "idx_email") {
		t.Fatalf("users table missing idx_email index")
	}

	if len(orders.ForeignKeys) == 0 {
		t.Fatalf("orders table should have foreign keys")
	}
	if !hasForeignKeyTo(orders, usersTable) {
		t.Fatalf("orders table missing FK to %s", usersTable)
	}
}

func TestSchemaDump_Pg_RealDB(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	drv, ok := db.Get("pg")
	if !ok {
		t.Fatal("pg driver not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("failed to open pg: %v", xe)
	}
	defer conn.Close()

	suffix := time.Now().UnixNano()
	schema := fmt.Sprintf("xsql_schema_%d", suffix)
	usersTable := "users"
	ordersTable := "orders"
	prefix := "xsql_"

	// 清理旧 schema
	_, _ = conn.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))

	// 创建 schema 与表
	_, err := conn.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema))
	if err != nil {
		t.Fatalf("create schema failed: %v", err)
	}

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s.%s (
			id BIGSERIAL PRIMARY KEY,
			email TEXT NOT NULL,
			created_at TIMESTAMPTZ NULL
		)
	`, schema, prefix+usersTable))
	if err != nil {
		t.Fatalf("create users table failed: %v", err)
	}

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX idx_email ON %s.%s (email)
	`, schema, prefix+usersTable))
	if err != nil {
		t.Fatalf("create index failed: %v", err)
	}

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s.%s (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			amount NUMERIC(10,2) NOT NULL,
			CONSTRAINT fk_%s_user FOREIGN KEY (user_id) REFERENCES %s.%s(id)
		)
	`, schema, prefix+ordersTable, prefix+ordersTable, schema, prefix+usersTable))
	if err != nil {
		t.Fatalf("create orders table failed: %v", err)
	}

	t.Cleanup(func() {
		_, _ = conn.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
	})

	info, xe := db.DumpSchema(ctx, "pg", conn, db.SchemaOptions{
		TablePattern: prefix + "*",
	})
	if xe != nil {
		t.Fatalf("DumpSchema error: %v", xe)
	}
	if info.Database == "" {
		t.Fatalf("database name is empty")
	}

	users := findTableWithSchema(info.Tables, schema, prefix+usersTable)
	orders := findTableWithSchema(info.Tables, schema, prefix+ordersTable)
	if users == nil || orders == nil {
		t.Fatalf("missing tables in schema dump: users=%v orders=%v", users != nil, orders != nil)
	}

	if !hasColumn(users, "id", true) {
		t.Fatalf("users table missing primary key column 'id'")
	}
	if len(users.Indexes) == 0 {
		t.Fatalf("users table should have indexes")
	}
	if !hasIndex(users, "idx_email") {
		t.Fatalf("users table missing idx_email index")
	}

	if len(orders.ForeignKeys) == 0 {
		t.Fatalf("orders table should have foreign keys")
	}
	if !hasForeignKeyTo(orders, prefix+usersTable) {
		t.Fatalf("orders table missing FK to %s", prefix+usersTable)
	}
}

func findTable(tables []db.Table, name string) *db.Table {
	for i := range tables {
		if tables[i].Name == name {
			return &tables[i]
		}
	}
	return nil
}

func findTableWithSchema(tables []db.Table, schema, name string) *db.Table {
	for i := range tables {
		if tables[i].Schema == schema && tables[i].Name == name {
			return &tables[i]
		}
	}
	return nil
}

func hasColumn(table *db.Table, name string, primary bool) bool {
	for _, c := range table.Columns {
		if c.Name == name && c.PrimaryKey == primary {
			return true
		}
	}
	return false
}

func hasIndex(table *db.Table, indexName string) bool {
	for _, idx := range table.Indexes {
		if idx.Name == indexName {
			return true
		}
	}
	return false
}

func hasForeignKeyTo(table *db.Table, referencedTable string) bool {
	for _, fk := range table.ForeignKeys {
		if strings.EqualFold(fk.ReferencedTable, referencedTable) {
			return true
		}
	}
	return false
}

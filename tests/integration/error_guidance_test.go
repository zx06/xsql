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

	_ "github.com/zx06/xsql/internal/db/mysql" // register MySQL driver
	_ "github.com/zx06/xsql/internal/db/pg"    // register PostgreSQL driver
)

type errorGuidanceCase struct {
	name         string
	driver       string
	dsnEnv       string
	dbType       string
	dsnOverride  string
	openOnly     bool
	sql          string
	wantCode     errors.Code
	wantCategory string
	wantHintSub  string
	checkDialect string
}

func TestErrorGuidance(t *testing.T) {
	mysqlDSN := os.Getenv("XSQL_TEST_MYSQL_DSN")
	pgDSN := os.Getenv("XSQL_TEST_PG_DSN")
	if mysqlDSN == "" && pgDSN == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN and XSQL_TEST_PG_DSN not set")
	}

	cases := []errorGuidanceCase{
		{
			name: "mysql_unknown_table", driver: "mysql", dsnEnv: "XSQL_TEST_MYSQL_DSN", dbType: "mysql",
			sql: "SELECT * FROM definitely_nonexistent_table_12345",
			wantCode: errors.CodeDBExecFailed, wantCategory: "unknown_table", wantHintSub: "schema",
			checkDialect: "mysql",
		},
		{
			name: "mysql_syntax_error", driver: "mysql", dsnEnv: "XSQL_TEST_MYSQL_DSN", dbType: "mysql",
			sql: "SELECT 1 +",
			wantCode: errors.CodeDBExecFailed, wantCategory: "syntax_error", wantHintSub: "syntax",
		},
		{
			name: "mysql_read_only_write", driver: "mysql", dsnEnv: "XSQL_TEST_MYSQL_DSN", dbType: "mysql",
			sql: "DELETE FROM nonexistent_table_xsql_ro",
			wantCode: errors.CodeROBlocked, wantCategory: "read_only_blocked", wantHintSub: "unsafe_allow_write",
		},
		{
			name: "mysql_unrecognized_typo", driver: "mysql", dsnEnv: "XSQL_TEST_MYSQL_DSN", dbType: "mysql",
			sql: "SELEC 1",
			wantCode: errors.CodeROBlocked, wantCategory: "unrecognized_statement", wantHintSub: "spelling",
		},
		{
			name: "mysql_auth_failed", driver: "mysql", dsnEnv: "XSQL_TEST_MYSQL_DSN", dbType: "mysql",
			openOnly: true, dsnOverride: badMySQLDSN(mysqlDSN),
			wantCode: errors.CodeDBConnectFailed, wantCategory: "auth_failed", wantHintSub: "keyring",
		},
		{
			name: "pg_unknown_table", driver: "pg", dsnEnv: "XSQL_TEST_PG_DSN", dbType: "pg",
			sql: "SELECT * FROM definitely_nonexistent_table_12345",
			wantCode: errors.CodeDBExecFailed, wantCategory: "unknown_table", wantHintSub: "schema",
		},
		{
			name: "pg_syntax_error", driver: "pg", dsnEnv: "XSQL_TEST_PG_DSN", dbType: "pg",
			sql: "SELECT 1 +",
			wantCode: errors.CodeDBExecFailed, wantCategory: "syntax_error", wantHintSub: "syntax",
		},
		{
			name: "pg_read_only_write", driver: "pg", dsnEnv: "XSQL_TEST_PG_DSN", dbType: "pg",
			sql: "DELETE FROM nonexistent_table_xsql_ro",
			wantCode: errors.CodeROBlocked, wantCategory: "read_only_blocked", wantHintSub: "unsafe_allow_write",
		},
		{
			name: "pg_unrecognized_typo", driver: "pg", dsnEnv: "XSQL_TEST_PG_DSN", dbType: "pg",
			sql: "SELEC 1",
			wantCode: errors.CodeROBlocked, wantCategory: "unrecognized_statement", wantHintSub: "spelling",
		},
		{
			name: "pg_auth_failed", driver: "pg", dsnEnv: "XSQL_TEST_PG_DSN", dbType: "pg",
			openOnly: true, dsnOverride: badPgDSN(pgDSN),
			wantCode: errors.CodeDBConnectFailed, wantCategory: "auth_failed", wantHintSub: "Authentication",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runErrorGuidanceCase(t, tc)
		})
	}
}

func runErrorGuidanceCase(t *testing.T, tc errorGuidanceCase) {
	t.Helper()

	dsn := tc.dsnOverride
	if dsn == "" {
		dsn = os.Getenv(tc.dsnEnv)
	}
	if dsn == "" {
		t.Skipf("%s not set", tc.dsnEnv)
	}

	drv, ok := db.Get(tc.driver)
	if !ok {
		t.Fatalf("driver %q not registered", tc.driver)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if tc.openOnly {
		assertErrorGuidance(t, xe, tc.wantCode, tc.wantCategory, tc.wantHintSub)
		return
	}
	if xe != nil {
		t.Fatalf("open: %v", xe)
	}
	defer conn.Close()

	_, xe = db.Query(ctx, conn, tc.sql, db.QueryOptions{DBType: tc.dbType})
	assertErrorGuidance(t, xe, tc.wantCode, tc.wantCategory, tc.wantHintSub)
	if tc.checkDialect != "" && xe != nil && xe.Details["dialect"] != tc.checkDialect {
		t.Errorf("dialect = %v, want %q", xe.Details["dialect"], tc.checkDialect)
	}
}

func assertErrorGuidance(t *testing.T, xe *errors.XError, wantCode errors.Code, wantCategory, wantHintSub string) {
	t.Helper()
	if xe == nil {
		t.Fatal("expected error")
	}
	if xe.Code != wantCode {
		t.Fatalf("code = %s, want %s", xe.Code, wantCode)
	}
	if xe.Details == nil {
		t.Fatal("expected error details")
	}
	cat, _ := xe.Details["category"].(string)
	if cat != wantCategory {
		t.Errorf("category = %q, want %q", cat, wantCategory)
	}
	hint, _ := xe.Details["hint"].(string)
	if hint == "" {
		t.Error("expected non-empty hint")
	} else if wantHintSub != "" && !strings.Contains(strings.ToLower(hint), strings.ToLower(wantHintSub)) {
		t.Errorf("hint %q should mention %q", hint, wantHintSub)
	}
	if _, ok := xe.Details["retryable"].(bool); !ok {
		t.Error("expected retryable bool in details")
	}
}

func badMySQLDSN(good string) string {
	if good == "" {
		return ""
	}
	if i := strings.Index(good, "@"); i > 0 {
		if j := strings.Index(good, ":"); j > 0 && j < i {
			return good[:j+1] + "xsql_wrong_password@" + good[i+1:]
		}
	}
	return good + "_invalid"
}

func badPgDSN(good string) string {
	if good == "" {
		return ""
	}
	return strings.Replace(good, "postgres:postgres@", "postgres:xsql_wrong_password@", 1)
}

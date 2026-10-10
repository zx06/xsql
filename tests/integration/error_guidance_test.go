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
	"github.com/zx06/xsql/internal/errors"

	_ "github.com/zx06/xsql/internal/db/mysql" // register MySQL driver
	_ "github.com/zx06/xsql/internal/db/pg"    // register PostgreSQL driver
)

type errorGuidanceEngine struct {
	label        string
	driver       string
	dsnEnv       string
	dbType       string
	dsn          string
	badDSN       func(string) string
	authHintSub  string
}

type errorGuidanceScenario struct {
	suffix       string
	sql          string
	openOnly     bool
	useBadDSN    bool
	wantCode     errors.Code
	wantCategory string
	wantHintSub  string
	checkDialect bool
}

var errorGuidanceScenarios = []errorGuidanceScenario{
	{
		suffix: "unknown_table", sql: "SELECT * FROM definitely_nonexistent_table_12345",
		wantCode: errors.CodeDBExecFailed, wantCategory: "unknown_table", wantHintSub: "schema", checkDialect: true,
	},
	{
		suffix: "syntax_error", sql: "SELECT 1 +",
		wantCode: errors.CodeDBExecFailed, wantCategory: "syntax_error", wantHintSub: "syntax",
	},
	{
		suffix: "read_only_write", sql: "DELETE FROM nonexistent_table_xsql_ro",
		wantCode: errors.CodeROBlocked, wantCategory: "read_only_blocked", wantHintSub: "unsafe_allow_write",
	},
	{
		suffix: "unrecognized_typo", sql: "SELEC 1",
		wantCode: errors.CodeROBlocked, wantCategory: "unrecognized_statement", wantHintSub: "spelling",
	},
	{
		suffix: "auth_failed", openOnly: true, useBadDSN: true,
		wantCode: errors.CodeDBConnectFailed, wantCategory: "auth_failed",
	},
}

func TestErrorGuidance(t *testing.T) {
	engines := []errorGuidanceEngine{
		{
			label: "mysql", driver: "mysql", dsnEnv: "XSQL_TEST_MYSQL_DSN", dbType: "mysql",
			dsn: os.Getenv("XSQL_TEST_MYSQL_DSN"), badDSN: badMySQLDSN, authHintSub: "keyring",
		},
		{
			label: "pg", driver: "pg", dsnEnv: "XSQL_TEST_PG_DSN", dbType: "pg",
			dsn: os.Getenv("XSQL_TEST_PG_DSN"), badDSN: badPgDSN, authHintSub: "Authentication",
		},
	}

	var runnable int
	for _, eng := range engines {
		if eng.dsn != "" {
			runnable++
		}
	}
	if runnable == 0 {
		t.Skip("XSQL_TEST_MYSQL_DSN and XSQL_TEST_PG_DSN not set")
	}

	for _, eng := range engines {
		if eng.dsn == "" {
			continue
		}
		for _, sc := range errorGuidanceScenarios {
			t.Run(fmt.Sprintf("%s_%s", eng.label, sc.suffix), func(t *testing.T) {
				runErrorGuidanceCase(t, guidanceCaseFrom(eng, sc))
			})
		}
	}
}

func guidanceCaseFrom(eng errorGuidanceEngine, sc errorGuidanceScenario) errorGuidanceCase {
	hintSub := sc.wantHintSub
	if sc.useBadDSN {
		hintSub = eng.authHintSub
	}
	tc := errorGuidanceCase{
		driver:       eng.driver,
		dsnEnv:       eng.dsnEnv,
		dbType:       eng.dbType,
		openOnly:     sc.openOnly,
		sql:          sc.sql,
		wantCode:     sc.wantCode,
		wantCategory: sc.wantCategory,
		wantHintSub:  hintSub,
	}
	if sc.useBadDSN {
		tc.dsnOverride = eng.badDSN(eng.dsn)
	}
	if sc.checkDialect {
		tc.checkDialect = eng.dbType
	}
	return tc
}

type errorGuidanceCase struct {
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

//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
)

func assertGuidance(t *testing.T, xe *errors.XError, wantCategory string, wantHintSub string) {
	t.Helper()
	if xe == nil {
		t.Fatal("expected error")
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
	} else if wantHintSub != "" && !containsInsensitive(hint, wantHintSub) {
		t.Errorf("hint %q should mention %q", hint, wantHintSub)
	}
	if _, ok := xe.Details["retryable"].(bool); !ok {
		t.Error("expected retryable bool in details")
	}
}

func containsInsensitive(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndexInsensitive(s, sub) >= 0)
}

func stringIndexInsensitive(s, sub string) int {
	ls := len(s)
	lsub := len(sub)
	for i := 0; i+lsub <= ls; i++ {
		if equalFoldASCII(s[i:i+lsub], sub) {
			return i
		}
	}
	return -1
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func TestErrorGuidance_MySQL(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}
	runMySQLGuidanceCases(t, dsn)
}

func runMySQLGuidanceCases(t *testing.T, dsn string) {
	drv, _ := db.Get("mysql")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, xe := drv.Open(ctx, db.ConnOptions{DSN: dsn})
	if xe != nil {
		t.Fatalf("open: %v", xe)
	}
	defer conn.Close()

	t.Run("unknown_table", func(t *testing.T) {
		_, xe := db.Query(ctx, conn, "SELECT * FROM definitely_nonexistent_table_12345", db.QueryOptions{DBType: "mysql"})
		if xe == nil || xe.Code != errors.CodeDBExecFailed {
			t.Fatalf("expected exec failed, got %v", xe)
		}
		assertGuidance(t, xe, "unknown_table", "schema dump")
		if xe.Details["dialect"] != "mysql" {
			t.Errorf("dialect = %v", xe.Details["dialect"])
		}
	})

	t.Run("syntax_error", func(t *testing.T) {
		_, xe := db.Query(ctx, conn, "SELECT 1 +", db.QueryOptions{DBType: "mysql"})
		if xe == nil || xe.Code != errors.CodeDBExecFailed {
			t.Fatalf("expected exec failed, got %v", xe)
		}
		assertGuidance(t, xe, "syntax_error", "syntax")
	})

	t.Run("read_only_blocked", func(t *testing.T) {
		_, xe := db.Query(ctx, conn, "DELETE FROM nonexistent_table_xsql_ro", db.QueryOptions{DBType: "mysql"})
		if xe == nil || xe.Code != errors.CodeROBlocked {
			t.Fatalf("expected RO blocked, got %v", xe)
		}
		assertGuidance(t, xe, "read_only_blocked", "unsafe_allow_write")
	})
}

func TestErrorGuidance_PostgreSQL(t *testing.T) {
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

	t.Run("unknown_table", func(t *testing.T) {
		_, xe := db.Query(ctx, conn, "SELECT * FROM definitely_nonexistent_table_12345", db.QueryOptions{DBType: "pg"})
		if xe == nil || xe.Code != errors.CodeDBExecFailed {
			t.Fatalf("expected exec failed, got %v", xe)
		}
		assertGuidance(t, xe, "unknown_table", "schema dump")
	})

	t.Run("syntax_error", func(t *testing.T) {
		_, xe := db.Query(ctx, conn, "SELECT 1 +", db.QueryOptions{DBType: "pg"})
		if xe == nil || xe.Code != errors.CodeDBExecFailed {
			t.Fatalf("expected exec failed, got %v", xe)
		}
		assertGuidance(t, xe, "syntax_error", "syntax")
	})

	t.Run("read_only_blocked", func(t *testing.T) {
		_, xe := db.Query(ctx, conn, "DELETE FROM nonexistent_table_xsql_ro", db.QueryOptions{DBType: "pg"})
		if xe == nil || xe.Code != errors.CodeROBlocked {
			t.Fatalf("expected RO blocked, got %v", xe)
		}
		assertGuidance(t, xe, "read_only_blocked", "unsafe_allow_write")
	})
}

package errors

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyMySQLError(t *testing.T) {
	tests := []struct {
		name       string
		number     uint16
		wantCat    DBCategory
		wantCode   string
		wantRetry  bool
		wantHintSub string
	}{
		{"unknown_table", 1146, CategoryUnknownTable, "1146", false, "schema dump"},
		{"unknown_column", 1054, CategoryUnknownColumn, "1054", false, "schema dump"},
		{"syntax", 1064, CategorySyntaxError, "1064", false, "syntax"},
		{"read_only_tx", 1792, CategoryReadOnlyViolation, "1792", false, "unsafe_allow_write"},
		{"connection", 2003, CategoryConnection, "2003", true, "host/port"},
		{"timeout", 1205, CategoryTimeout, "1205", true, "LIMIT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := classifyMySQLError(&mysql.MySQLError{Number: tt.number, Message: "driver message"})
			if c.Category != tt.wantCat {
				t.Errorf("category = %q, want %q", c.Category, tt.wantCat)
			}
			if c.NativeCode != tt.wantCode {
				t.Errorf("native_code = %q, want %q", c.NativeCode, tt.wantCode)
			}
			if c.NativeCodeKind != "mysql_errno" {
				t.Errorf("native_code_kind = %q", c.NativeCodeKind)
			}
			if c.Retryable != tt.wantRetry {
				t.Errorf("retryable = %v, want %v", c.Retryable, tt.wantRetry)
			}
			if tt.wantHintSub != "" && !containsFold(c.Hint, tt.wantHintSub) {
				t.Errorf("hint %q should contain %q", c.Hint, tt.wantHintSub)
			}
		})
	}
}

func TestClassifyPgError(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		wantCat    DBCategory
		wantRetry  bool
		wantHintSub string
	}{
		{"unknown_table", "42P01", CategoryUnknownTable, false, "schema dump"},
		{"unknown_column", "42703", CategoryUnknownColumn, false, "schema dump"},
		{"syntax", "42601", CategorySyntaxError, false, "syntax"},
		{"read_only", "25006", CategoryReadOnlyViolation, false, "unsafe_allow_write"},
		{"connection", "08006", CategoryConnection, true, "PostgreSQL"},
		{"timeout", "57014", CategoryTimeout, true, "LIMIT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := classifyPgError(&pgconn.PgError{Code: tt.code, Message: "driver message"})
			if c.Category != tt.wantCat {
				t.Errorf("category = %q, want %q", c.Category, tt.wantCat)
			}
			if c.NativeCode != tt.code {
				t.Errorf("native_code = %q, want %q", c.NativeCode, tt.code)
			}
			if c.NativeCodeKind != "pg_sqlstate" {
				t.Errorf("native_code_kind = %q", c.NativeCodeKind)
			}
			if c.Retryable != tt.wantRetry {
				t.Errorf("retryable = %v, want %v", c.Retryable, tt.wantRetry)
			}
			if tt.wantHintSub != "" && !containsFold(c.Hint, tt.wantHintSub) {
				t.Errorf("hint %q should contain %q", c.Hint, tt.wantHintSub)
			}
		})
	}
}

func TestClassifyDriverError_Chain(t *testing.T) {
	inner := &mysql.MySQLError{Number: 1146, Message: "Table 'db.t' doesn't exist"}
	xe := Wrap(CodeDBExecFailed, "query failed", map[string]any{"dialect": "mysql"}, inner)
	if xe.Details["category"] != string(CategoryUnknownTable) {
		t.Errorf("expected unknown_table category, got %v", xe.Details["category"])
	}
	if xe.Details["native_code"] != "1146" {
		t.Errorf("expected native_code 1146, got %v", xe.Details["native_code"])
	}
}

func TestReadOnlyBlockedGuidance(t *testing.T) {
	xe := New(CodeROBlocked, "write blocked", map[string]any{"reason": "forbidden_keyword: DELETE"})
	if xe.Details["category"] != "read_only_blocked" {
		t.Errorf("category = %v", xe.Details["category"])
	}
	if xe.Details["retryable"] != false {
		t.Errorf("retryable should be false")
	}
	hint, _ := xe.Details["hint"].(string)
	if hint == "" || !containsFold(hint, "DELETE") && !containsFold(hint, "write keyword") {
		t.Errorf("unexpected hint: %q", hint)
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

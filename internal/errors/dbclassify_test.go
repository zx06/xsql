package errors

import (
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

type classifyWant struct {
	category DBCategory
	code     string
	kind     string
	retry    bool
	hintSub  string
}

func assertMySQLClassify(t *testing.T, number uint16, want classifyWant) {
	t.Helper()
	c := classifyMySQLError(&mysql.MySQLError{Number: number, Message: "driver message"})
	assertClassification(t, c, want)
}

func assertPgClassify(t *testing.T, code string, want classifyWant) {
	t.Helper()
	c := classifyPgError(&pgconn.PgError{Code: code, Message: "driver message"})
	want.code = code
	want.kind = "pg_sqlstate"
	assertClassification(t, c, want)
}

func assertClassification(t *testing.T, c dbClassification, want classifyWant) {
	t.Helper()
	if c.Category != want.category {
		t.Errorf("category = %q, want %q", c.Category, want.category)
	}
	if want.code != "" && c.NativeCode != want.code {
		t.Errorf("native_code = %q, want %q", c.NativeCode, want.code)
	}
	if want.kind != "" && c.NativeCodeKind != want.kind {
		t.Errorf("native_code_kind = %q, want %q", c.NativeCodeKind, want.kind)
	}
	if c.Retryable != want.retry {
		t.Errorf("retryable = %v, want %v", c.Retryable, want.retry)
	}
	if want.hintSub != "" && !strings.Contains(strings.ToLower(c.Hint), strings.ToLower(want.hintSub)) {
		t.Errorf("hint %q should contain %q", c.Hint, want.hintSub)
	}
}

func TestClassifyMySQLError(t *testing.T) {
	cases := map[string]struct {
		number uint16
		want   classifyWant
	}{
		"unknown_table":   {1146, classifyWant{CategoryUnknownTable, "1146", "mysql_errno", false, "schema_list_tables"}},
		"unknown_db":      {1049, classifyWant{CategoryUnknownTable, "1049", "mysql_errno", false, "schema dump"}},
		"unknown_column":  {1054, classifyWant{CategoryUnknownColumn, "1054", "mysql_errno", false, "schema dump"}},
		"syntax":          {1064, classifyWant{CategorySyntaxError, "1064", "mysql_errno", false, "syntax"}},
		"auth_failed":     {1045, classifyWant{CategoryAuthFailed, "1045", "mysql_errno", false, "keyring"}},
		"permission":      {1142, classifyWant{CategoryPermissionDenied, "1142", "mysql_errno", false, "privileges"}},
		"read_only_tx":    {1792, classifyWant{CategoryReadOnlyViolation, "1792", "mysql_errno", false, "unsafe_allow_write"}},
		"duplicate_key":   {1062, classifyWant{CategoryDuplicateKey, "1062", "mysql_errno", false, "Duplicate"}},
		"connection":      {2003, classifyWant{CategoryConnection, "2003", "mysql_errno", true, "MySQL"}},
		"timeout":         {1205, classifyWant{CategoryTimeout, "1205", "mysql_errno", true, "LIMIT"}},
		"other":           {9999, classifyWant{CategoryOther, "9999", "mysql_errno", false, "native_code"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertMySQLClassify(t, tc.number, tc.want)
		})
	}
}

func TestClassifyPgError(t *testing.T) {
	cases := map[string]struct {
		code string
		want classifyWant
	}{
		"unknown_table":  {"42P01", classifyWant{CategoryUnknownTable, "", "", false, "schema dump"}},
		"unknown_column": {"42703", classifyWant{CategoryUnknownColumn, "", "", false, "schema dump"}},
		"syntax":         {"42601", classifyWant{CategorySyntaxError, "", "", false, "syntax"}},
		"auth_failed":    {"28P01", classifyWant{CategoryAuthFailed, "", "", false, "keyring"}},
		"auth_invalid":   {"28000", classifyWant{CategoryAuthFailed, "", "", false, "Authentication"}},
		"permission":     {"42501", classifyWant{CategoryPermissionDenied, "", "", false, "privileges"}},
		"read_only":      {"25006", classifyWant{CategoryReadOnlyViolation, "", "", false, "unsafe_allow_write"}},
		"duplicate_key":  {"23505", classifyWant{CategoryDuplicateKey, "", "", false, "Duplicate"}},
		"connection":     {"08006", classifyWant{CategoryConnection, "", "", true, "PostgreSQL"}},
		"timeout":        {"57014", classifyWant{CategoryTimeout, "", "", true, "LIMIT"}},
		"serialization":  {"40001", classifyWant{CategoryTimeout, "", "", true, "Serialization"}},
		"other":          {"99999", classifyWant{CategoryOther, "", "", false, "native_code"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertPgClassify(t, tc.code, tc.want)
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

func TestClassifyDriverError_AuthOnConnect(t *testing.T) {
	inner := &mysql.MySQLError{Number: 1045, Message: "Access denied for user 'root'@'localhost'"}
	xe := Wrap(CodeDBConnectFailed, "failed to ping mysql", map[string]any{"dialect": "mysql"}, inner)
	if xe.Details["category"] != string(CategoryAuthFailed) {
		t.Errorf("category = %v", xe.Details["category"])
	}
	if xe.Details["retryable"] != false {
		t.Error("auth should not be retryable")
	}
}

func TestClassifyMySQLMessagePaths(t *testing.T) {
	c, ok := classifyMySQLMessage("Access denied for user")
	if !ok || c.Category != CategoryAuthFailed {
		t.Fatalf("auth message: %+v ok=%v", c, ok)
	}
	c, ok = classifyMySQLMessage("Unknown column 'x' in 'field list'")
	if !ok || c.Category != CategoryUnknownColumn {
		t.Fatalf("column: %+v ok=%v", c, ok)
	}
}

func TestClassifyPgMessagePaths(t *testing.T) {
	c, ok := classifyPgMessage(`password authentication failed for user "postgres"`)
	if !ok || c.Category != CategoryAuthFailed {
		t.Fatalf("auth: %+v ok=%v", c, ok)
	}
	c, ok = classifyPgMessage(`relation "missing" does not exist`)
	if !ok || c.Category != CategoryUnknownTable {
		t.Fatalf("table: %+v ok=%v", c, ok)
	}
}

func TestClassifyNetworkErrorBranches(t *testing.T) {
	c, ok := classifyNetworkError(&net.DNSError{IsTimeout: true})
	if !ok || c.Category != CategoryTimeout {
		t.Fatalf("timeout: %+v", c)
	}
	c, ok = classifyNetworkError(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED})
	if !ok || c.Category != CategoryConnection {
		t.Fatalf("conn: %+v", c)
	}
	c, ok = classifyNetworkError(syscall.ECONNRESET)
	if !ok || c.Category != CategoryConnection {
		t.Fatalf("reset: %+v", c)
	}
}

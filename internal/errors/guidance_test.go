package errors

import (
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMergeDetails_PreservesExisting(t *testing.T) {
	base := map[string]any{"reason": "forbidden_keyword: INSERT", "hint": "custom"}
	add := map[string]any{"hint": "ignored", "retryable": false}
	merged := mergeDetails(base, add)
	if merged["hint"] != "custom" {
		t.Errorf("hint = %v, want custom", merged["hint"])
	}
	if merged["retryable"] != false {
		t.Errorf("retryable not merged")
	}
}

func TestConnectGuidance_Auth(t *testing.T) {
	xe := Wrap(CodeDBAuthFailed, "auth failed", map[string]any{"dialect": "mysql"}, errors.New("access denied for user"))
	if xe.Details["category"] != string(CategoryAuthFailed) {
		t.Errorf("category = %v", xe.Details["category"])
	}
	if xe.Details["retryable"] != false {
		t.Errorf("retryable should be false")
	}
}

func TestConnectGuidance_Network(t *testing.T) {
	xe := Wrap(CodeDBConnectFailed, "ping failed", map[string]any{"dialect": "pg"}, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED})
	if xe.Details["category"] != string(CategoryConnection) {
		t.Errorf("category = %v", xe.Details["category"])
	}
	if xe.Details["retryable"] != true {
		t.Errorf("retryable should be true")
	}
}

func TestSSHGuidance_Codes(t *testing.T) {
	auth := Wrap(CodeSSHAuthFailed, "ssh auth", nil, errors.New("unable to authenticate"))
	if auth.Details["retryable"] != false {
		t.Error("ssh auth should not be retryable")
	}
	hostkey := New(CodeSSHHostKeyMismatch, "host key", nil)
	if !containsFold(hostkey.Details["hint"].(string), "host key") {
		t.Errorf("hint = %v", hostkey.Details["hint"])
	}
}

func TestConfigGuidance(t *testing.T) {
	xe := New(CodeCfgNotFound, "missing", nil)
	if !containsFold(xe.Details["hint"].(string), "config init") {
		t.Errorf("hint = %v", xe.Details["hint"])
	}
}

func TestConfigGuidance_TableNotFound(t *testing.T) {
	xe := New(CodeCfgInvalid, "table not found", map[string]any{
		"reason": "table_not_found",
		"schema": "public",
		"name":   "missing_tbl",
	})
	if xe.Details["category"] != string(CategoryUnknownTable) {
		t.Errorf("category = %v", xe.Details["category"])
	}
	hint, _ := xe.Details["hint"].(string)
	if !containsFold(hint, "schema_list_tables") {
		t.Errorf("hint = %q", hint)
	}
}

func TestClassifyNetworkTimeout(t *testing.T) {
	err := &net.DNSError{IsTimeout: true, Err: "timeout"}
	c, ok := classifyNetworkError(err)
	if !ok || c.Category != CategoryTimeout || !c.Retryable {
		t.Fatalf("unexpected classification: %+v ok=%v", c, ok)
	}
}

func TestClassifyPgMessageFallback(t *testing.T) {
	c, ok := classifyPgMessage(`relation "missing" does not exist`)
	if !ok || c.Category != CategoryUnknownTable {
		t.Fatalf("got %+v ok=%v", c, ok)
	}
}

func TestDBExecGuidance_Fallback(t *testing.T) {
	xe := Wrap(CodeDBExecFailed, "query failed", map[string]any{"dialect": "mysql"}, errors.New("generic failure"))
	if xe.Details["category"] != string(CategoryOther) {
		t.Errorf("category = %v", xe.Details["category"])
	}
}

func TestReadOnlyGuidance_DefaultHint(t *testing.T) {
	xe := New(CodeROBlocked, "blocked", map[string]any{"reason": "forbidden_share_lock"})
	if xe.Details["category"] != categoryReadOnlyBlocked {
		t.Errorf("category = %v", xe.Details["category"])
	}
	if xe.Details["hint"] == "" {
		t.Fatal("expected hint")
	}
}

func TestReadOnlyGuidance_NoKeyword(t *testing.T) {
	xe := New(CodeROBlocked, "blocked", map[string]any{"reason": "no_keyword"})
	if xe.Details["category"] != categoryUnrecognizedStmt {
		t.Errorf("category = %v", xe.Details["category"])
	}
	hint, _ := xe.Details["hint"].(string)
	if !containsFold(hint, "spelling") {
		t.Errorf("hint = %q", hint)
	}
}

func TestReadOnlyGuidance_TypoSELEC(t *testing.T) {
	xe := New(CodeROBlocked, "blocked", map[string]any{"reason": "no_keyword"})
	if containsFold(xe.Details["hint"].(string), "unsafe_allow_write") {
		t.Error("typo should not suggest enabling writes")
	}
}

func TestReadOnlyGuidance_ForbiddenStart(t *testing.T) {
	xe := New(CodeROBlocked, "blocked", map[string]any{"reason": "forbidden_start: INSERT"})
	if xe.Details["category"] != categoryReadOnlyBlocked {
		t.Errorf("category = %v", xe.Details["category"])
	}
	if !containsFold(xe.Details["hint"].(string), "non-read-only") {
		t.Errorf("hint = %v", xe.Details["hint"])
	}
}

func TestIsReadOnlyUnrecognizedReason(t *testing.T) {
	if !isReadOnlyUnrecognizedReason("tokenize_error: bad") {
		t.Error("expected tokenize_error prefix")
	}
	if isReadOnlyUnrecognizedReason("forbidden_keyword: DELETE") {
		t.Error("write keyword should not be unrecognized")
	}
}

func TestReadOnlyBlockedGuidance_ForbiddenKeyword(t *testing.T) {
	xe := New(CodeROBlocked, "write blocked", map[string]any{"reason": "forbidden_keyword: DELETE"})
	if xe.Details["category"] != categoryReadOnlyBlocked {
		t.Errorf("category = %v", xe.Details["category"])
	}
	hint, _ := xe.Details["hint"].(string)
	if !containsFold(hint, "write keyword") {
		t.Errorf("unexpected hint: %q", hint)
	}
}

func TestConnectGuidance_PGAuthSQLState(t *testing.T) {
	inner := &pgconn.PgError{Code: "28P01", Message: "password authentication failed"}
	xe := Wrap(CodeDBConnectFailed, "failed to ping pg", map[string]any{"dialect": "pg"}, inner)
	if xe.Details["category"] != string(CategoryAuthFailed) {
		t.Errorf("category = %v", xe.Details["category"])
	}
}

func TestDialectFromDetails(t *testing.T) {
	if dialectFromDetails(map[string]any{"db": "postgresql"}) != "pg" {
		t.Error("expected pg from db field")
	}
}

func TestSSHDialGuidance(t *testing.T) {
	xe := Wrap(CodeSSHDialFailed, "dial", map[string]any{"host": "bastion"}, errors.New("connection reset"))
	if xe.Details["retryable"] != true {
		t.Errorf("retryable = %v", xe.Details["retryable"])
	}
}

func TestMergeDetails_NilBase(t *testing.T) {
	merged := mergeDetails(nil, map[string]any{"hint": "x"})
	if merged["hint"] != "x" {
		t.Errorf("merged = %v", merged)
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

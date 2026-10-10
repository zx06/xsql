package errors

import (
	"errors"
	"net"
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
	if xe.Details["category"] != string(CategoryPermissionDenied) {
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
	xe := New(CodeROBlocked, "blocked", map[string]any{"reason": "not_select"})
	if xe.Details["hint"] == "" {
		t.Fatal("expected hint")
	}
}

func TestDialectFromDetails(t *testing.T) {
	if dialectFromDetails(map[string]any{"db": "postgresql"}) != "pg" {
		t.Error("expected pg from db field")
	}
}

func TestClassifyMySQLMessagePaths(t *testing.T) {
	c, ok := classifyMySQLMessage("Unknown column 'x' in 'field list'")
	if !ok || c.Category != CategoryUnknownColumn {
		t.Fatalf("got %+v ok=%v", c, ok)
	}
}

func TestClassifyPgError_Default(t *testing.T) {
	c := classifyPgError(&pgconn.PgError{Code: "99999", Message: "weird"})
	if c.Category != CategoryOther {
		t.Errorf("category = %q", c.Category)
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

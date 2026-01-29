package errors

import (
	stderrors "errors"
	"testing"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		code Code
		want ExitCode
	}{
		{CodeCfgNotFound, ExitConfig},
		{CodeCfgInvalid, ExitConfig},
		{CodeSecretNotFound, ExitConfig},
		{CodeSSHDialFailed, ExitConnect},
		{CodeSSHAuthFailed, ExitConnect},
		{CodeSSHHostKeyMismatch, ExitConnect},
		{CodeDBConnectFailed, ExitConnect},
		{CodeDBAuthFailed, ExitConnect},
		{CodeDBDriverUnsupported, ExitConnect},
		{CodeROBlocked, ExitReadOnly},
		{CodeDBExecFailed, ExitDBExec},
		{CodeInternal, ExitInternal},
		{Code("UNKNOWN_CODE"), ExitInternal}, // unknown code
	}
	for _, tc := range cases {
		if got := ExitCodeFor(tc.code); got != tc.want {
			t.Errorf("ExitCodeFor(%s)=%d want %d", tc.code, got, tc.want)
		}
	}
}

func TestXError_Error(t *testing.T) {
	// Without cause
	xe := New(CodeCfgInvalid, "test message", nil)
	expected := "XSQL_CFG_INVALID: test message"
	if xe.Error() != expected {
		t.Errorf("Error()=%q, want %q", xe.Error(), expected)
	}

	// With cause
	cause := stderrors.New("underlying error")
	xe = Wrap(CodeDBExecFailed, "query failed", nil, cause)
	expected = "XSQL_DB_EXEC_FAILED: query failed: underlying error"
	if xe.Error() != expected {
		t.Errorf("Error()=%q, want %q", xe.Error(), expected)
	}

	// Nil error
	var nilErr *XError
	if nilErr.Error() != "" {
		t.Errorf("nil XError.Error() should return empty string")
	}
}

func TestXError_Unwrap(t *testing.T) {
	cause := stderrors.New("cause")
	xe := Wrap(CodeDBExecFailed, "msg", nil, cause)
	if xe.Unwrap() != cause {
		t.Error("Unwrap should return cause")
	}

	xe2 := New(CodeCfgInvalid, "msg", nil)
	if xe2.Unwrap() != nil {
		t.Error("Unwrap should return nil when no cause")
	}
}

func TestXError_Details(t *testing.T) {
	details := map[string]any{"key": "value", "count": 42}
	xe := New(CodeCfgInvalid, "msg", details)
	if xe.Details["key"] != "value" {
		t.Error("Details should contain key")
	}
	if xe.Details["count"] != 42 {
		t.Error("Details should contain count")
	}
}

func TestAs(t *testing.T) {
	xe := New(CodeCfgInvalid, "test", nil)
	got, ok := As(xe)
	if !ok || got != xe {
		t.Error("As should return XError")
	}

	// Wrapped error
	wrapped := stderrors.Join(stderrors.New("prefix"), xe)
	got, ok = As(wrapped)
	if !ok || got != xe {
		t.Error("As should unwrap to find XError")
	}

	// Non-XError
	_, ok = As(stderrors.New("plain error"))
	if ok {
		t.Error("As should return false for non-XError")
	}
}

func TestAllCodes(t *testing.T) {
	codes := AllCodes()
	if len(codes) != 12 {
		t.Errorf("AllCodes() should return 12 codes, got %d", len(codes))
	}

	// Check for duplicates
	seen := make(map[Code]bool)
	for _, c := range codes {
		if seen[c] {
			t.Errorf("Duplicate code: %s", c)
		}
		seen[c] = true
	}
}

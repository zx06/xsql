package app

import "testing"

func TestBuildSpecHasSchemaVersion(t *testing.T) {
	a := New("dev")
	s := a.BuildSpec()
	if s.SchemaVersion != 1 {
		t.Fatalf("schema_version=%d want 1", s.SchemaVersion)
	}
	if len(s.ErrorCodes) == 0 {
		t.Fatalf("expected error codes")
	}
}

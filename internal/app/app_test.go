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
	if len(s.Commands) == 0 || len(s.Commands[0].Flags) == 0 {
		t.Fatalf("expected commands/flags")
	}
	seenProfile := false
	for _, f := range s.Commands[0].Flags {
		if f.Name == "profile" && f.Env == "XSQL_PROFILE" {
			seenProfile = true
		}
	}
	if !seenProfile {
		t.Fatalf("expected profile flag in spec")
	}
}

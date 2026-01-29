package app

import "testing"

func TestBuildSpecHasSchemaVersion(t *testing.T) {
	a := New("dev", "abc123", "2024-01-01")
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

func TestVersionInfo(t *testing.T) {
	a := New("v1.0.0", "abc123", "2024-01-01")
	v := a.VersionInfo()
	if v.Version != "v1.0.0" {
		t.Errorf("version=%s want v1.0.0", v.Version)
	}
	if v.Commit != "abc123" {
		t.Errorf("commit=%s want abc123", v.Commit)
	}
	if v.Date != "2024-01-01" {
		t.Errorf("date=%s want 2024-01-01", v.Date)
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_DefaultPaths_NoConfig(t *testing.T) {
	tmp := t.TempDir()
	got, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp})
	if xe != nil {
		t.Fatalf("unexpected err: %v", xe)
	}
	if got.ConfigPath != "" {
		t.Fatalf("expected empty config path")
	}
	if got.Format != "auto" {
		t.Fatalf("format=%q want auto", got.Format)
	}
}

func TestResolve_ExplicitConfigMissingIsError(t *testing.T) {
	tmp := t.TempDir()
	_, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp, ConfigPath: "no_such.yaml"})
	if xe == nil {
		t.Fatalf("expected error")
	}
	if xe.Code != "XSQL_CFG_NOT_FOUND" {
		t.Fatalf("code=%s", xe.Code)
	}
}

func TestResolve_ProfileAndFormatPrecedence(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte("profiles:\n  default:\n    format: yaml\n  dev:\n    format: json\n")
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	// No CLI/ENV profile -> profiles.default selected
	got, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp})
	if xe != nil {
		t.Fatal(xe)
	}
	if got.ProfileName != "default" || got.Format != "yaml" {
		t.Fatalf("got profile=%q format=%q", got.ProfileName, got.Format)
	}

	// ENV overrides config
	got, xe = Resolve(Options{WorkDir: tmp, HomeDir: tmp, EnvFormat: "json"})
	if xe != nil {
		t.Fatal(xe)
	}
	if got.Format != "json" {
		t.Fatalf("format=%q want json", got.Format)
	}

	// CLI overrides ENV
	got, xe = Resolve(Options{WorkDir: tmp, HomeDir: tmp, EnvFormat: "yaml", CLIFormat: "table", CLIFormatSet: true})
	if xe != nil {
		t.Fatal(xe)
	}
	if got.Format != "table" {
		t.Fatalf("format=%q want table", got.Format)
	}

	// CLI profile overrides default
	got, xe = Resolve(Options{WorkDir: tmp, HomeDir: tmp, CLIProfile: "dev", CLIProfileSet: true})
	if xe != nil {
		t.Fatal(xe)
	}
	if got.ProfileName != "dev" || got.Format != "json" {
		t.Fatalf("got profile=%q format=%q", got.ProfileName, got.Format)
	}
}

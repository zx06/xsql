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

func TestResolve_SSHProxyResolution(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_rsa
profiles:
  prod:
    db: mysql
    host: db.internal
    ssh_proxy: bastion
`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	got, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp, CLIProfile: "prod", CLIProfileSet: true})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if got.Profile.SSHConfig == nil {
		t.Fatal("expected SSHConfig to be resolved")
	}
	if got.Profile.SSHConfig.Host != "bastion.example.com" {
		t.Errorf("expected host=bastion.example.com, got %q", got.Profile.SSHConfig.Host)
	}
	if got.Profile.SSHConfig.User != "admin" {
		t.Errorf("expected user=admin, got %q", got.Profile.SSHConfig.User)
	}
}

func TestResolve_SSHProxyNotFound(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`profiles:
  prod:
    db: mysql
    host: db.internal
    ssh_proxy: nonexistent
`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	_, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp, CLIProfile: "prod", CLIProfileSet: true})
	if xe == nil {
		t.Fatal("expected error for missing ssh_proxy")
	}
	if xe.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected code=XSQL_CFG_INVALID, got %s", xe.Code)
	}
}

func TestResolve_DefaultPort_MySQL(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`profiles:
  mysql_db:
    db: mysql
    host: localhost
`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	got, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp, CLIProfile: "mysql_db", CLIProfileSet: true})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if got.Profile.Port != 3306 {
		t.Errorf("expected default port 3306 for MySQL, got %d", got.Profile.Port)
	}
}

func TestResolve_DefaultPort_PostgreSQL(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`profiles:
  pg_db:
    db: pg
    host: localhost
`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	got, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp, CLIProfile: "pg_db", CLIProfileSet: true})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if got.Profile.Port != 5432 {
		t.Errorf("expected default port 5432 for PostgreSQL, got %d", got.Profile.Port)
	}
}

func TestResolve_Port_PreservedWhenSpecified(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`profiles:
  custom_port:
    db: mysql
    host: localhost
    port: 13306
`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	got, xe := Resolve(Options{WorkDir: tmp, HomeDir: tmp, CLIProfile: "custom_port", CLIProfileSet: true})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if got.Profile.Port != 13306 {
		t.Errorf("expected port 13306 to be preserved, got %d", got.Profile.Port)
	}
}

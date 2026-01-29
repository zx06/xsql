package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_NoConfig(t *testing.T) {
	tmp := t.TempDir()
	cfg, path, xe := LoadConfig(Options{WorkDir: tmp, HomeDir: tmp})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if path != "" {
		t.Fatalf("expected empty path, got %q", path)
	}
	if cfg.Profiles == nil {
		t.Fatal("expected non-nil Profiles map")
	}
	if len(cfg.Profiles) != 0 {
		t.Fatalf("expected empty profiles, got %d", len(cfg.Profiles))
	}
}

func TestLoadConfig_ExplicitConfigMissing(t *testing.T) {
	tmp := t.TempDir()
	_, _, xe := LoadConfig(Options{WorkDir: tmp, HomeDir: tmp, ConfigPath: "no_such.yaml"})
	if xe == nil {
		t.Fatal("expected error")
	}
	if xe.Code != "XSQL_CFG_NOT_FOUND" {
		t.Fatalf("expected XSQL_CFG_NOT_FOUND, got %s", xe.Code)
	}
}

func TestLoadConfig_WorkDirConfig(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`profiles:
  dev:
    db: mysql
    host: localhost
    port: 3306
  prod:
    db: pg
    host: db.example.com
    port: 5432
`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	file, cfgPath, xe := LoadConfig(Options{WorkDir: tmp, HomeDir: tmp})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if cfgPath != path {
		t.Fatalf("expected path %q, got %q", path, cfgPath)
	}
	if len(file.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(file.Profiles))
	}
	if _, ok := file.Profiles["dev"]; !ok {
		t.Fatal("expected 'dev' profile")
	}
	if _, ok := file.Profiles["prod"]; !ok {
		t.Fatal("expected 'prod' profile")
	}

	// Verify profile details
	dev := file.Profiles["dev"]
	if dev.DB != "mysql" {
		t.Errorf("expected db=mysql, got %q", dev.DB)
	}
	if dev.Host != "localhost" {
		t.Errorf("expected host=localhost, got %q", dev.Host)
	}
	if dev.Port != 3306 {
		t.Errorf("expected port=3306, got %d", dev.Port)
	}

	prod := file.Profiles["prod"]
	if prod.DB != "pg" {
		t.Errorf("expected db=pg, got %q", prod.DB)
	}
}

func TestLoadConfig_HomeDirConfig(t *testing.T) {
	workDir := t.TempDir()
	homeDir := t.TempDir()

	// Only create config in home dir
	cfgDir := filepath.Join(homeDir, ".config", "xsql")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`profiles:
  home:
    db: mysql
`)
	path := filepath.Join(cfgDir, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	file, cfgPath, xe := LoadConfig(Options{WorkDir: workDir, HomeDir: homeDir})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if cfgPath != path {
		t.Fatalf("expected path %q, got %q", path, cfgPath)
	}
	if _, ok := file.Profiles["home"]; !ok {
		t.Fatal("expected 'home' profile")
	}
}

func TestLoadConfig_WorkDirTakesPrecedence(t *testing.T) {
	workDir := t.TempDir()
	homeDir := t.TempDir()

	// Create config in both locations
	workCfg := []byte(`profiles:
  work:
    db: mysql
`)
	if err := os.WriteFile(filepath.Join(workDir, "xsql.yaml"), workCfg, 0o600); err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(homeDir, ".config", "xsql")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	homeCfg := []byte(`profiles:
  home:
    db: pg
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "xsql.yaml"), homeCfg, 0o600); err != nil {
		t.Fatal(err)
	}

	file, cfgPath, xe := LoadConfig(Options{WorkDir: workDir, HomeDir: homeDir})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if cfgPath != filepath.Join(workDir, "xsql.yaml") {
		t.Fatalf("expected work dir config, got %q", cfgPath)
	}
	if _, ok := file.Profiles["work"]; !ok {
		t.Fatal("expected 'work' profile from work dir")
	}
	if _, ok := file.Profiles["home"]; ok {
		t.Fatal("should not have 'home' profile from home dir")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`invalid: yaml: syntax: [`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, xe := LoadConfig(Options{WorkDir: tmp, HomeDir: tmp})
	if xe == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if xe.Code != "XSQL_CFG_INVALID" {
		t.Fatalf("expected XSQL_CFG_INVALID, got %s", xe.Code)
	}
}

func TestLoadConfig_ExplicitPath(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`profiles:
  explicit:
    db: mysql
`)
	customPath := filepath.Join(tmp, "custom.yaml")
	if err := os.WriteFile(customPath, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	file, cfgPath, xe := LoadConfig(Options{ConfigPath: customPath})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if cfgPath != customPath {
		t.Fatalf("expected path %q, got %q", customPath, cfgPath)
	}
	if _, ok := file.Profiles["explicit"]; !ok {
		t.Fatal("expected 'explicit' profile")
	}
}

func TestLoadConfig_ProfileWithSSHProxy(t *testing.T) {
	tmp := t.TempDir()
	cfg := []byte(`ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_rsa
    skip_host_key: false

profiles:
  ssh-test:
    db: mysql
    host: db.internal
    port: 3306
    user: root
    password: keyring:test/password
    database: testdb
    ssh_proxy: bastion
`)
	path := filepath.Join(tmp, "xsql.yaml")
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	file, _, xe := LoadConfig(Options{WorkDir: tmp, HomeDir: tmp})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	p := file.Profiles["ssh-test"]
	if p.SSHProxy != "bastion" {
		t.Errorf("expected ssh_proxy=bastion, got %q", p.SSHProxy)
	}

	proxy, ok := file.SSHProxies["bastion"]
	if !ok {
		t.Fatal("expected bastion proxy to exist")
	}
	if proxy.Host != "bastion.example.com" {
		t.Errorf("expected proxy host=bastion.example.com, got %q", proxy.Host)
	}
	if proxy.Port != 22 {
		t.Errorf("expected proxy port=22, got %d", proxy.Port)
	}
	if proxy.User != "admin" {
		t.Errorf("expected proxy user=admin, got %q", proxy.User)
	}
	if proxy.IdentityFile != "~/.ssh/id_rsa" {
		t.Errorf("expected proxy identity_file=~/.ssh/id_rsa, got %q", proxy.IdentityFile)
	}
	if p.Password != "keyring:test/password" {
		t.Errorf("expected password=keyring:test/password, got %q", p.Password)
	}
}

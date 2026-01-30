//go:build e2e

package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// xsql profile list Tests
// ============================================================================

func TestProfile_List_JSON(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    description: "开发环境数据库"
    db: mysql
    host: localhost
    port: 3306
  staging:
    description: "预发布环境"
    db: pg
    host: staging.example.com
    port: 5432
    unsafe_allow_write: true
  prod:
    description: "生产环境数据库"
    db: mysql
    host: prod.example.com
    port: 3306
`)

	stdout, _, exitCode := runXSQL(t, "profile", "list", "--config", config, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout: %s", exitCode, stdout)
	}

	type profileInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		DB          string `json:"db"`
		Mode        string `json:"mode"`
	}
	var resp struct {
		OK            bool `json:"ok"`
		SchemaVersion int  `json:"schema_version"`
		Data          struct {
			ConfigPath string        `json:"config_path"`
			Profiles   []profileInfo `json:"profiles"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", resp.SchemaVersion)
	}
	if resp.Data.ConfigPath == "" {
		t.Error("expected config_path to be set")
	}
	if len(resp.Data.Profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(resp.Data.Profiles))
	}

	// Check all profiles are present with correct mode and description
	profiles := make(map[string]profileInfo)
	for _, p := range resp.Data.Profiles {
		profiles[p.Name] = p
	}
	if p, ok := profiles["dev"]; !ok || p.Mode != "read-only" || p.Description != "开发环境数据库" {
		t.Errorf("expected dev profile with read-only mode and description, got %+v", p)
	}
	if p, ok := profiles["staging"]; !ok || p.Mode != "read-write" || p.Description != "预发布环境" {
		t.Errorf("expected staging profile with read-write mode and description, got %+v", p)
	}
	if p, ok := profiles["prod"]; !ok || p.Mode != "read-only" || p.Description != "生产环境数据库" {
		t.Errorf("expected prod profile with read-only mode and description, got %+v", p)
	}
}

func TestProfile_List_YAML(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test1:
    description: "Test DB 1"
    db: mysql
  test2:
    description: "Test DB 2"
    db: pg
`)

	stdout, _, exitCode := runXSQL(t, "profile", "list", "--config", config, "--format", "yaml")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	type profileInfo struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		DB          string `yaml:"db"`
		Mode        string `yaml:"mode"`
	}
	var resp struct {
		OK   bool `yaml:"ok"`
		Data struct {
			Profiles []profileInfo `yaml:"profiles"`
		} `yaml:"data"`
	}
	if err := yaml.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid YAML: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if len(resp.Data.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(resp.Data.Profiles))
	}
	// Verify descriptions
	for _, p := range resp.Data.Profiles {
		if p.Description == "" {
			t.Errorf("expected description for profile %s", p.Name)
		}
	}
}

func TestProfile_List_Table(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    description: "Dev Database"
    db: mysql
  prod:
    description: "Prod Database"
    db: pg
`)

	stdout, _, exitCode := runXSQL(t, "profile", "list", "--config", config, "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Table format should contain profile names
	if !strings.Contains(stdout, "dev") {
		t.Errorf("expected output to contain 'dev', got: %s", stdout)
	}
	if !strings.Contains(stdout, "prod") {
		t.Errorf("expected output to contain 'prod', got: %s", stdout)
	}
	// Table format should contain DESCRIPTION header
	if !strings.Contains(stdout, "DESCRIPTION") {
		t.Errorf("expected table header to contain 'DESCRIPTION', got: %s", stdout)
	}
	// Table format should contain description values
	if !strings.Contains(stdout, "Dev Database") {
		t.Errorf("expected output to contain 'Dev Database', got: %s", stdout)
	}
	if !strings.Contains(stdout, "Prod Database") {
		t.Errorf("expected output to contain 'Prod Database', got: %s", stdout)
	}
}

func TestProfile_List_EmptyConfig(t *testing.T) {
	config := createTempConfig(t, `profiles: {}`)

	stdout, _, exitCode := runXSQL(t, "profile", "list", "--config", config, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Profiles []string `json:"profiles"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if len(resp.Data.Profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(resp.Data.Profiles))
	}
}

func TestProfile_List_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Run with explicitly specified non-existent config
	stdout, _, exitCode := runXSQL(t, "profile", "list", "--config", filepath.Join(tmpDir, "nonexistent.yaml"), "--format", "json")

	if exitCode != 2 {
		t.Errorf("expected exit code 2, got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != "XSQL_CFG_NOT_FOUND" {
		t.Errorf("expected XSQL_CFG_NOT_FOUND, got %s", resp.Error.Code)
	}
}

// ============================================================================
// xsql profile show Tests
// ============================================================================

func TestProfile_Show_JSON(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    description: "开发环境 MySQL"
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: secret123
    database: mydb
    unsafe_allow_write: true
`)

	stdout, _, exitCode := runXSQL(t, "profile", "show", "dev", "--config", config, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout: %s", exitCode, stdout)
	}

	var resp struct {
		OK            bool `json:"ok"`
		SchemaVersion int  `json:"schema_version"`
		Data          struct {
			ConfigPath       string `json:"config_path"`
			Name             string `json:"name"`
			Description      string `json:"description"`
			DB               string `json:"db"`
			Host             string `json:"host"`
			Port             int    `json:"port"`
			User             string `json:"user"`
			Password         string `json:"password"`
			Database         string `json:"database"`
			UnsafeAllowWrite bool   `json:"unsafe_allow_write"`
			AllowPlaintext   bool   `json:"allow_plaintext"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Name != "dev" {
		t.Errorf("expected name=dev, got %q", resp.Data.Name)
	}
	if resp.Data.Description != "开发环境 MySQL" {
		t.Errorf("expected description='开发环境 MySQL', got %q", resp.Data.Description)
	}
	if resp.Data.DB != "mysql" {
		t.Errorf("expected db=mysql, got %q", resp.Data.DB)
	}
	if resp.Data.Host != "localhost" {
		t.Errorf("expected host=localhost, got %q", resp.Data.Host)
	}
	if resp.Data.Port != 3306 {
		t.Errorf("expected port=3306, got %d", resp.Data.Port)
	}
	if resp.Data.User != "root" {
		t.Errorf("expected user=root, got %q", resp.Data.User)
	}
	if resp.Data.Database != "mydb" {
		t.Errorf("expected database=mydb, got %q", resp.Data.Database)
	}
	if !resp.Data.UnsafeAllowWrite {
		t.Error("expected unsafe_allow_write=true")
	}

	// Password should be masked
	if resp.Data.Password != "***" {
		t.Errorf("expected password to be masked as '***', got %q", resp.Data.Password)
	}
}

func TestProfile_Show_WithSSHProxy(t *testing.T) {
	config := createTempConfig(t, `ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_rsa

profiles:
  prod:
    db: pg
    host: db.internal
    port: 5432
    user: app
    password: keyring:prod/password
    database: production
    ssh_proxy: bastion
`)

	stdout, _, exitCode := runXSQL(t, "profile", "show", "prod", "--config", config, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		Data struct {
			SSHProxy        string `json:"ssh_proxy"`
			SSHHost         string `json:"ssh_host"`
			SSHPort         int    `json:"ssh_port"`
			SSHUser         string `json:"ssh_user"`
			SSHIdentityFile string `json:"ssh_identity_file"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Data.SSHProxy != "bastion" {
		t.Errorf("expected ssh_proxy=bastion, got %q", resp.Data.SSHProxy)
	}
	if resp.Data.SSHHost != "bastion.example.com" {
		t.Errorf("expected ssh_host=bastion.example.com, got %q", resp.Data.SSHHost)
	}
	if resp.Data.SSHPort != 22 {
		t.Errorf("expected ssh_port=22, got %d", resp.Data.SSHPort)
	}
	if resp.Data.SSHUser != "admin" {
		t.Errorf("expected ssh_user=admin, got %q", resp.Data.SSHUser)
	}
	if resp.Data.SSHIdentityFile != "~/.ssh/id_rsa" {
		t.Errorf("expected ssh_identity_file=~/.ssh/id_rsa, got %q", resp.Data.SSHIdentityFile)
	}
}

func TestProfile_Show_YAML(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: localhost
`)

	stdout, _, exitCode := runXSQL(t, "profile", "show", "test", "--config", config, "--format", "yaml")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool `yaml:"ok"`
		Data struct {
			Name string `yaml:"name"`
			DB   string `yaml:"db"`
		} `yaml:"data"`
	}
	if err := yaml.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid YAML: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Name != "test" {
		t.Errorf("expected name=test, got %q", resp.Data.Name)
	}
}

func TestProfile_Show_Table(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: localhost
`)

	stdout, _, exitCode := runXSQL(t, "profile", "show", "dev", "--config", config, "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Table format should contain profile details
	if !strings.Contains(stdout, "mysql") {
		t.Errorf("expected output to contain 'mysql', got: %s", stdout)
	}
	if !strings.Contains(stdout, "localhost") {
		t.Errorf("expected output to contain 'localhost', got: %s", stdout)
	}
}

func TestProfile_Show_NotFound(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
`)

	stdout, _, exitCode := runXSQL(t, "profile", "show", "nonexistent", "--config", config, "--format", "json")

	if exitCode != 2 {
		t.Errorf("expected exit code 2 (config error), got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected XSQL_CFG_INVALID, got %s", resp.Error.Code)
	}
}

func TestProfile_Show_MissingArg(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
`)

	_, stderr, exitCode := runXSQL(t, "profile", "show", "--config", config, "--format", "json")

	// Should fail because profile name is required
	if exitCode == 0 {
		t.Error("expected non-zero exit code when profile name is missing")
	}

	// Should have usage error in stderr or stdout
	if !strings.Contains(stderr, "requires") && !strings.Contains(stderr, "argument") &&
		!strings.Contains(stderr, "accepts") {
		t.Logf("stderr: %s (expected error about missing argument)", stderr)
	}
}

func TestProfile_Show_DSNMasked(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "root:secret@tcp(localhost:3306)/testdb"
`)

	stdout, _, exitCode := runXSQL(t, "profile", "show", "test", "--config", config, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		Data struct {
			DSN string `json:"dsn"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// DSN should be masked
	if resp.Data.DSN != "***" {
		t.Errorf("expected dsn to be masked as '***', got %q", resp.Data.DSN)
	}
}

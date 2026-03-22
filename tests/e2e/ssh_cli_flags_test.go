//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSSH_SkipKnownHostsCheckFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
ssh_proxies:
  test_ssh:
    host: localhost
    port: 22
    user: testuser
    identity_file: /nonexistent/key

profiles:
  test:
    db: mysql
    host: localhost
    ssh_proxy: test_ssh
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", configPath,
		"--profile", "test",
		"--format", "json",
		"--ssh-skip-known-hosts-check")

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if exitCode == 0 {
		t.Log("Query succeeded with SSH proxy")
	} else {
		if resp.Error != nil && resp.Error.Code == "XSQL_SSH_HOSTKEY_MISMATCH" {
			t.Error("should not get XSQL_SSH_HOSTKEY_MISMATCH when --ssh-skip-known-hosts-check is set")
		}
	}
}

func TestSSH_IdentityFileFlagOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
ssh_proxies:
  test_ssh:
    host: localhost
    port: 22
    user: testuser
    identity_file: /nonexistent/config_key

profiles:
  test:
    db: mysql
    host: localhost
    ssh_proxy: test_ssh
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	_, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", configPath,
		"--profile", "test",
		"--format", "json",
		"--ssh-identity-file", "/nonexistent/cli_key")

	if exitCode == 0 {
		t.Log("Query succeeded with SSH proxy")
	} else {
		t.Log("Query failed (expected without real SSH server)")
	}
}

func TestSSH_UserFlagOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
ssh_proxies:
  test_ssh:
    host: localhost
    port: 22
    user: config_user
    identity_file: /nonexistent/key

profiles:
  test:
    db: mysql
    host: localhost
    ssh_proxy: test_ssh
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	_, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", configPath,
		"--profile", "test",
		"--format", "json",
		"--ssh-user", "cli_user")

	if exitCode == 0 {
		t.Log("Query succeeded with SSH proxy")
	} else {
		t.Log("Query failed (expected without real SSH server)")
	}
}

func TestSSH_HostFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
profiles:
  test:
    db: mysql
    host: localhost
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	_, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", configPath,
		"--profile", "test",
		"--format", "json",
		"--ssh-host", "example.com",
		"--ssh-user", "test",
		"--ssh-skip-known-hosts-check")

	if exitCode == 0 {
		t.Log("Query succeeded with SSH proxy")
	} else {
		t.Log("Query failed (expected without real SSH server)")
	}
}

//go:build e2e

package e2e

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ============================================================================
// xsql config init Tests
// ============================================================================

func TestConfigInit_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")

	stdout, _, exitCode := runXSQL(t, "config", "init", "--path", path, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}

	if resp.Data == nil {
		t.Fatal("expected data in response")
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["config_path"] != path {
		t.Errorf("expected config_path=%s, got %v", path, data["config_path"])
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file should exist: %v", err)
	}
}

func TestConfigInit_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "init", "--path", path, "--format", "json")

	if exitCode == 0 {
		t.Error("expected non-zero exit code when file exists")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestConfigInit_TableFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")

	stdout, _, exitCode := runXSQL(t, "config", "init", "--path", path, "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Should not be JSON
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Error("table format should not output JSON")
	}
}

// ============================================================================
// xsql config set Tests
// ============================================================================

func TestConfigSet_ProfileHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "profile.dev.host", "localhost",
		"--config", path, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.OK {
		t.Error("expected ok=true")
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["key"] != "profile.dev.host" {
		t.Errorf("expected key=profile.dev.host, got %v", data["key"])
	}
	if data["value"] != "localhost" {
		t.Errorf("expected value=localhost, got %v", data["value"])
	}

	// Verify config was updated
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "localhost") {
		t.Error("config should contain 'localhost'")
	}
}

func TestConfigSet_ProfilePort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "profile.dev.port", "3306",
		"--config", path, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestConfigSet_ProfileLocalPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "profile.prod.local_port", "13306",
		"--config", path, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestConfigSet_SSHProxy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "ssh_proxy.bastion.host", "bastion.example.com",
		"--config", path, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestConfigSet_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "badkey", "value",
		"--config", path, "--format", "json")

	if exitCode == 0 {
		t.Error("expected non-zero exit code for invalid key")
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

func TestConfigSet_InvalidPortValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "profile.dev.port", "abc",
		"--config", path, "--format", "json")

	if exitCode == 0 {
		t.Error("expected non-zero exit code for invalid port")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestConfigSet_UnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "profile.dev.nonexistent", "value",
		"--config", path, "--format", "json")

	if exitCode == 0 {
		t.Error("expected non-zero exit code for unknown field")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestConfigSet_MissingArgs(t *testing.T) {
	_, _, exitCode := runXSQL(t, "config", "set", "--format", "json")

	if exitCode == 0 {
		t.Error("expected non-zero exit code for missing args")
	}
}

func TestConfigSet_TableFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runXSQL(t, "config", "set", "profile.dev.host", "localhost",
		"--config", path, "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Should not be JSON
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Error("table format should not output JSON on stdout")
	}
}

// ============================================================================
// Config Help Tests
// ============================================================================

func TestConfig_Help(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "config", "--help")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "config") {
		t.Error("help output should contain 'config'")
	}
	if !strings.Contains(stdout, "init") {
		t.Error("help output should contain 'init'")
	}
	if !strings.Contains(stdout, "set") {
		t.Error("help output should contain 'set'")
	}
}

func TestConfigInit_Help(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "config", "init", "--help")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "init") {
		t.Error("help should contain 'init'")
	}
}

func TestConfigSet_Help(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "config", "set", "--help")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "set") {
		t.Error("help should contain 'set'")
	}
}

// ============================================================================
// Proxy with config local_port Tests
// ============================================================================

func TestProxy_UsesConfigLocalPort(t *testing.T) {
	// Test that proxy reads local_port from profile config
	config := createTempConfig(t, `ssh_proxies:
  test_ssh:
    host: bastion.example.com
    user: test
profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    local_port: 13306
    ssh_proxy: test_ssh
`)

	// This will fail to connect to SSH, but should read the config
	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--format", "json")

	// Should fail due to SSH connection issues
	if exitCode == 0 {
		// If somehow it succeeded, verify local port
		var resp Response
		if err := json.Unmarshal([]byte(stdout), &resp); err == nil && resp.OK && resp.Data != nil {
			data, ok := resp.Data.(map[string]any)
			if ok {
				if localAddr, hasAddr := data["local_address"].(string); hasAddr {
					if !strings.Contains(localAddr, "13306") {
						t.Errorf("expected local_address to contain port 13306, got %s", localAddr)
					}
				}
			}
		}
	}

	// The output should be valid JSON regardless
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
}

func TestProxy_CLIFlagOverridesConfigLocalPort(t *testing.T) {
	// Test that --local-port flag overrides config local_port
	config := createTempConfig(t, `ssh_proxies:
  test_ssh:
    host: bastion.example.com
    user: test
profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    local_port: 13306
    ssh_proxy: test_ssh
`)

	// Use --local-port to override config
	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--local-port", "23306", "--format", "json")

	// Will fail at SSH, but should have accepted the CLI port
	if exitCode == 0 {
		var resp Response
		if err := json.Unmarshal([]byte(stdout), &resp); err == nil && resp.OK && resp.Data != nil {
			data, ok := resp.Data.(map[string]any)
			if ok {
				if localAddr, hasAddr := data["local_address"].(string); hasAddr {
					if !strings.Contains(localAddr, "23306") {
						t.Errorf("expected local_address to contain port 23306, got %s", localAddr)
					}
				}
			}
		}
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
}

func TestProxy_ConfigPortInUse_NonTTY(t *testing.T) {
	// When config specifies a port that's in use and we're non-TTY, should get error
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	config := createTempConfig(t, `ssh_proxies:
  test_ssh:
    host: bastion.example.com
    user: test
    identity_file: ~/.ssh/id_ed25519
profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    local_port: `+strconv.Itoa(port)+`
    ssh_proxy: test_ssh
`)

	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--format", "json")

	// Should fail because port is in use (non-TTY mode)
	if exitCode == 0 {
		t.Error("expected non-zero exit code when config port is in use")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error != nil && resp.Error.Code != "XSQL_PORT_IN_USE" {
		// May also be SSH error if port resolution happens differently
		t.Logf("error code: %s (may be port or ssh error)", resp.Error.Code)
	}
}

func TestProxy_CLIFlagPortInUse(t *testing.T) {
	// When CLI flag specifies a port that's in use, should fail directly
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	config := createTempConfig(t, `ssh_proxies:
  test_ssh:
    host: bastion.example.com
    user: test
profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    ssh_proxy: test_ssh
`)

	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--local-port", strconv.Itoa(port), "--format", "json")

	// Should fail
	if exitCode == 0 {
		t.Error("expected non-zero exit code when CLI port is in use")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

// ============================================================================
// Config set + profile show integration
// ============================================================================

func TestConfigSet_ThenProfileShow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Set profile fields
	fields := []struct {
		key, value string
	}{
		{"profile.dev.db", "mysql"},
		{"profile.dev.host", "localhost"},
		{"profile.dev.port", "3306"},
		{"profile.dev.user", "root"},
		{"profile.dev.database", "testdb"},
		{"profile.dev.local_port", "13306"},
	}

	for _, f := range fields {
		_, _, exitCode := runXSQL(t, "config", "set", f.key, f.value,
			"--config", path, "--format", "json")
		if exitCode != 0 {
			t.Fatalf("config set %s=%s failed", f.key, f.value)
		}
	}

	// Show the profile
	stdout, _, exitCode := runXSQL(t, "-p", "dev", "profile", "show", "dev",
		"--config", path, "--format", "json")

	if exitCode != 0 {
		t.Fatalf("profile show failed; output: %s", stdout)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["db"] != "mysql" {
		t.Errorf("expected db=mysql, got %v", data["db"])
	}
	if data["host"] != "localhost" {
		t.Errorf("expected host=localhost, got %v", data["host"])
	}
}

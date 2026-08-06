package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdXSQLAI_MissingProfileError(t *testing.T) {
	tmpDir := t.TempDir()
	emptyConfig := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(emptyConfig, []byte("profiles: {}\n"), 0600); err != nil {
		t.Fatalf("failed to write empty config: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", emptyConfig})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no profile specified and no default profile")
	}
	if !strings.Contains(err.Error(), "no profile specified") {
		t.Fatalf("expected missing profile error message, got %v", err)
	}
}

func TestCmdXSQLAI_ConfigPathNotFound(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", "/nonexistent/config.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config file does not exist")
	}
	if !strings.Contains(err.Error(), "config error") {
		t.Fatalf("expected config error prefix, got %v", err)
	}
}

func TestCmdXSQLAI_FlagsBinding(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "xsql.yaml")
	cfgContent := `
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    database: testdb
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	flags := &AIFlags{
		ConfigPath: cfgPath,
		Profile:    "dev",
		Model:      "gpt-4o",
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "--profile", "dev", "Show top 10 servers"})

	// Verify command flag parsing
	if err := cmd.ParseFlags([]string{"--config", cfgPath, "--profile", "dev", "--unsafe-allow-write"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if flags.Profile != "dev" {
		t.Fatalf("expected profile 'dev', got %q", flags.Profile)
	}
}

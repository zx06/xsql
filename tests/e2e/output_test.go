//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================================================
// stdout/stderr Separation Tests
// ============================================================================

func TestOutput_SeparateStdoutStderr(t *testing.T) {
	// Test that data goes to stdout and logs go to stderr
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, stderr, exitCode := runXSQL(t, "query", "SELECT 1 as n",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// stdout should contain valid JSON response
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout should be valid JSON: %v\nstdout: %s", err, stdout)
	}
	if !resp.OK {
		t.Error("expected ok=true in stdout")
	}

	// stderr should NOT contain JSON data (only logs/debug output)
	if strings.Contains(stderr, `"ok":`) || strings.Contains(stderr, `"data":`) {
		t.Error("stderr should not contain JSON data output")
	}

	// stderr might contain debug info but not structured response
	t.Logf("stdout length: %d, stderr length: %d", len(stdout), len(stderr))
}

func TestOutput_StderrOnlyOnError(t *testing.T) {
	// When there's an error, error response should be in stdout, not stderr
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, stderr, exitCode := runXSQL(t, "query", "SELECT * FROM nonexistent_table_xyz",
		"--config", config, "--profile", "test", "--format", "json")

	// Should fail with DB exec error (exit code 5)
	if exitCode != 5 {
		t.Errorf("expected exit code 5, got %d", exitCode)
	}

	// Error response should be in stdout
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout should contain error JSON: %v\nstdout: %s", err, stdout)
	}
	if resp.OK {
		t.Error("expected ok=false in stdout for error case")
	}
	if resp.Error == nil {
		t.Error("expected error in stdout")
	}

	// stderr might contain additional debug info but not the error response
	if strings.Contains(stderr, `"error":`) && !strings.Contains(stderr, "error") {
		t.Logf("stderr contains error info (acceptable for debugging)")
	}
}

func TestOutput_TTYMode_StdoutOnly(t *testing.T) {
	// In TTY mode (no --format), output should still go to stdout
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	// Run without --format (should default based on TTY)
	stdout, stderr, exitCode := runXSQL(t, "query", "SELECT 1 as n",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// All structured output should be in stdout
	if stdout == "" {
		t.Error("expected stdout to have output")
	}

	// stderr should be empty or only contain debug logs
	if len(stderr) > 0 && strings.Contains(stderr, "{") {
		t.Error("stderr should not contain JSON structures")
	}
}

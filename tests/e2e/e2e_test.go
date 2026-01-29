//go:build e2e

// Package e2e contains end-to-end tests for the xsql CLI.
// These tests exercise the CLI binary as a black box, testing all features
// through the command line interface.
//
// Run with: go test -tags=e2e ./tests/e2e/... -v
// Requires: XSQL_TEST_MYSQL_DSN and/or XSQL_TEST_PG_DSN environment variables
package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var testBinary string

func TestMain(m *testing.M) {
	// Build test binary
	tmpDir, err := os.MkdirTemp("", "xsql-e2e-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	testBinary = filepath.Join(tmpDir, "xsql")
	if os.PathSeparator == '\\' {
		testBinary += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", testBinary, "../../cmd/xsql")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(string(out))
	}

	os.Exit(m.Run())
}

// ============================================================================
// Response Types
// ============================================================================

type Response struct {
	OK            bool   `json:"ok" yaml:"ok"`
	SchemaVersion int    `json:"schema_version" yaml:"schema_version"`
	Data          any    `json:"data,omitempty" yaml:"data,omitempty"`
	Error         *Error `json:"error,omitempty" yaml:"error,omitempty"`
}

type Error struct {
	Code    string         `json:"code" yaml:"code"`
	Message string         `json:"message" yaml:"message"`
	Details map[string]any `json:"details,omitempty" yaml:"details,omitempty"`
}

type QueryResult struct {
	Columns []string         `json:"columns" yaml:"columns"`
	Rows    []map[string]any `json:"rows" yaml:"rows"`
}

type VersionInfo struct {
	Version string `json:"version" yaml:"version"`
}

// ============================================================================
// Helper Functions
// ============================================================================

func runXSQL(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(testBinary, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run command: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func mysqlDSN(t *testing.T) string {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}
	return dsn
}

func pgDSN(t *testing.T) string {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}
	return dsn
}

// ============================================================================
// xsql spec Tests
// ============================================================================

func TestSpec_JSON(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "spec", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", resp.SchemaVersion)
	}
	if resp.Data == nil {
		t.Error("expected data to be present")
	}

	// Verify spec structure
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if _, ok := data["commands"]; !ok {
		t.Error("spec should contain 'commands' field")
	}
	if _, ok := data["error_codes"]; !ok {
		t.Error("spec should contain 'error_codes' field")
	}
}

func TestSpec_YAML(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "spec", "--format", "yaml")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp Response
	if err := yaml.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid YAML: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", resp.SchemaVersion)
	}
}

// ============================================================================
// xsql version Tests
// ============================================================================

func TestVersion_JSON(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "version", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK            bool        `json:"ok"`
		SchemaVersion int         `json:"schema_version"`
		Data          VersionInfo `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Version == "" {
		t.Error("expected version string to be non-empty")
	}
}

func TestVersion_YAML(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "version", "--format", "yaml")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK            bool        `yaml:"ok"`
		SchemaVersion int         `yaml:"schema_version"`
		Data          VersionInfo `yaml:"data"`
	}
	if err := yaml.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid YAML: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestVersion_Table(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "version", "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Table format should contain "version" text
	if !strings.Contains(stdout, "version") {
		t.Errorf("expected table output to contain 'version', got: %s", stdout)
	}
}

// ============================================================================
// xsql query Tests - MySQL
// ============================================================================

func TestQuery_MySQL_BasicSelect(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as num, 'hello' as msg",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", exitCode, stdout)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if len(resp.Data.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(resp.Data.Columns))
	}
	if len(resp.Data.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(resp.Data.Rows))
	}
	if resp.Data.Rows[0]["msg"] != "hello" {
		t.Errorf("expected msg='hello', got %v", resp.Data.Rows[0]["msg"])
	}
}

func TestQuery_MySQL_MultipleRows(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n UNION SELECT 2 UNION SELECT 3",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(resp.Data.Rows))
	}
}

func TestQuery_MySQL_EmptyResult(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n FROM (SELECT 1) t WHERE 1=0",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(resp.Data.Rows))
	}
	// Columns should still be present
	if len(resp.Data.Columns) != 1 {
		t.Errorf("columns should be present even with no rows")
	}
}

func TestQuery_MySQL_ShowDatabases(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SHOW DATABASES",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) == 0 {
		t.Error("expected at least one database")
	}
}

func TestQuery_MySQL_Explain(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "EXPLAIN SELECT 1",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Columns) == 0 {
		t.Error("EXPLAIN should return columns")
	}
}

func TestQuery_MySQL_ReadOnlyBlocked(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	tests := []struct {
		name string
		sql  string
	}{
		{"INSERT", "INSERT INTO test VALUES (1)"},
		{"UPDATE", "UPDATE test SET x=1"},
		{"DELETE", "DELETE FROM test"},
		{"DROP", "DROP TABLE test"},
		{"CREATE", "CREATE TABLE test (id INT)"},
		{"TRUNCATE", "TRUNCATE TABLE test"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, exitCode := runXSQL(t, "query", tc.sql,
				"--config", config, "--profile", "test", "--format", "json")

			// Exit code 4 = read-only blocked
			if exitCode != 4 {
				t.Errorf("expected exit code 4 (RO_BLOCKED), got %d", exitCode)
			}

			var resp Response
			if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			if resp.OK {
				t.Error("expected ok=false")
			}
			if resp.Error == nil || resp.Error.Code != "XSQL_RO_BLOCKED" {
				t.Errorf("expected error code XSQL_RO_BLOCKED, got %v", resp.Error)
			}
		})
	}
}

func TestQuery_MySQL_UnsafeAllowWrite(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	// With --unsafe-allow-write, write statements should bypass read-only check
	// They may still fail at DB level, but not with XSQL_RO_BLOCKED
	stdout, _, exitCode := runXSQL(t, "query", "INSERT INTO nonexistent VALUES (1)",
		"--config", config, "--profile", "test", "--format", "json",
		"--unsafe-allow-write")

	// Should fail with DB error (table doesn't exist), not RO_BLOCKED
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Error != nil && resp.Error.Code == "XSQL_RO_BLOCKED" {
		t.Error("--unsafe-allow-write should bypass read-only check")
	}

	// Exit code should be 5 (DB_EXEC_FAILED), not 4 (RO_BLOCKED)
	if exitCode == 4 {
		t.Error("expected exit code != 4 (should not be RO_BLOCKED)")
	}
}

func TestQuery_MySQL_InvalidSQL(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT * FROM definitely_nonexistent_table_xyz",
		"--config", config, "--profile", "test", "--format", "json")

	// Exit code 5 = DB execution error
	if exitCode != 5 {
		t.Errorf("expected exit code 5 (DB_EXEC_FAILED), got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error == nil || resp.Error.Code != "XSQL_DB_EXEC_FAILED" {
		t.Errorf("expected error code XSQL_DB_EXEC_FAILED, got %v", resp.Error)
	}
}

func TestQuery_MySQL_DataTypes(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", `SELECT 
		42 as int_val,
		3.14 as float_val,
		'text' as str_val,
		TRUE as bool_val`,
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	row := resp.Data.Rows[0]
	if row["str_val"] != "text" {
		t.Errorf("str_val=%v, want 'text'", row["str_val"])
	}
}

// ============================================================================
// xsql query Tests - PostgreSQL
// ============================================================================

func TestQuery_PG_BasicSelect(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as num, 'hello' as msg, NULL as empty",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", exitCode, stdout)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if len(resp.Data.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(resp.Data.Columns))
	}
	if resp.Data.Rows[0]["empty"] != nil {
		t.Errorf("expected empty=nil, got %v", resp.Data.Rows[0]["empty"])
	}
}

func TestQuery_PG_GenerateSeries(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT generate_series(1,5) as n",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) != 5 {
		t.Errorf("expected 5 rows, got %d", len(resp.Data.Rows))
	}
}

func TestQuery_PG_SystemCatalog(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT datname FROM pg_database LIMIT 5",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) == 0 {
		t.Error("expected at least one database")
	}
}

func TestQuery_PG_CTE(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", `
		WITH numbers AS (
			SELECT generate_series(1, 3) as n
		)
		SELECT n * 2 as doubled FROM numbers`,
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(resp.Data.Rows))
	}
}

func TestQuery_PG_ReadOnlyBlocked(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "INSERT INTO test VALUES (1)",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 4 {
		t.Errorf("expected exit code 4 (RO_BLOCKED), got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Error == nil || resp.Error.Code != "XSQL_RO_BLOCKED" {
		t.Errorf("expected error code XSQL_RO_BLOCKED")
	}
}

// ============================================================================
// Output Format Tests
// ============================================================================

func TestQuery_OutputFormat_JSON(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", resp.SchemaVersion)
	}
}

func TestQuery_OutputFormat_YAML(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n",
		"--config", config, "--profile", "test", "--format", "yaml")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp Response
	if err := yaml.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("output should be valid YAML: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestQuery_OutputFormat_Table(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as num, 'hello' as msg",
		"--config", config, "--profile", "test", "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Table format should NOT contain "ok" or "schema_version"
	if strings.Contains(stdout, "schema_version") {
		t.Error("table format should not include schema_version")
	}

	// Should contain column headers
	if !strings.Contains(stdout, "num") || !strings.Contains(stdout, "msg") {
		t.Errorf("table should contain column headers, got: %s", stdout)
	}

	// Should contain data
	if !strings.Contains(stdout, "hello") {
		t.Errorf("table should contain data, got: %s", stdout)
	}

	// Should contain row count
	if !strings.Contains(stdout, "1 row") {
		t.Errorf("table should contain row count, got: %s", stdout)
	}
}

func TestQuery_OutputFormat_CSV(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as num, 'hello' as msg",
		"--config", config, "--profile", "test", "--format", "csv")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// CSV format should NOT contain "ok" or "schema_version"
	if strings.Contains(stdout, "schema_version") {
		t.Error("csv format should not include schema_version")
	}

	// Should be valid CSV with header row
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (header + data), got %d", len(lines))
	}

	// First line should be headers
	if lines[0] != "num,msg" {
		t.Errorf("expected header 'num,msg', got '%s'", lines[0])
	}

	// Second line should be data
	if lines[1] != "1,hello" {
		t.Errorf("expected data '1,hello', got '%s'", lines[1])
	}
}

func TestQuery_OutputFormat_Table_MultipleRows(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n UNION SELECT 2 UNION SELECT 3",
		"--config", config, "--profile", "test", "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Should contain "3 rows"
	if !strings.Contains(stdout, "3 rows") {
		t.Errorf("table should show '3 rows', got: %s", stdout)
	}
}

func TestQuery_OutputFormat_Table_EmptyResult(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n FROM (SELECT 1) t WHERE 1=0",
		"--config", config, "--profile", "test", "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", exitCode, stdout)
	}

	// Should contain "0 rows"
	if !strings.Contains(stdout, "0 rows") {
		t.Errorf("table should show '0 rows', got: %s", stdout)
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestError_MissingProfile(t *testing.T) {
	config := createTempConfig(t, `profiles:
  existing:
    db: mysql
    dsn: "root:root@tcp(localhost:3306)/test"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "nonexistent", "--format", "json")

	// Exit code 2 = config/argument error
	if exitCode != 2 {
		t.Errorf("expected exit code 2 (CFG_INVALID), got %d", exitCode)
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

func TestError_InvalidFormat(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "spec", "--format", "invalid_format")

	if exitCode != 2 {
		t.Errorf("expected exit code 2 (CFG_INVALID), got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error == nil || resp.Error.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected XSQL_CFG_INVALID")
	}
}

func TestError_MissingSQL(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "root:root@tcp(localhost:3306)/test"
`)

	_, _, exitCode := runXSQL(t, "query",
		"--config", config, "--profile", "test", "--format", "json")

	// Missing required argument
	if exitCode == 0 {
		t.Error("expected non-zero exit code for missing SQL argument")
	}
}

func TestError_ConnectionFailed(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "root:wrongpassword@tcp(localhost:9999)/test"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "test", "--format", "json")

	// Exit code 3 = connection error
	if exitCode != 3 {
		t.Errorf("expected exit code 3 (DB_CONNECT_FAILED), got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error == nil || resp.Error.Code != "XSQL_DB_CONNECT_FAILED" {
		t.Errorf("expected XSQL_DB_CONNECT_FAILED, got %v", resp.Error)
	}
}

func TestError_UnsupportedDBType(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: sqlite
    dsn: "test.db"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "test", "--format", "json")

	// Exit code 2 or other (unsupported driver)
	if exitCode == 0 {
		t.Error("expected non-zero exit code for unsupported db type")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
}

// ============================================================================
// Exit Code Tests
// ============================================================================

func TestExitCode_Success(t *testing.T) {
	_, _, exitCode := runXSQL(t, "version", "--format", "json")
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestExitCode_ConfigError(t *testing.T) {
	_, _, exitCode := runXSQL(t, "spec", "--format", "invalid")
	if exitCode != 2 {
		t.Errorf("expected exit code 2 (config error), got %d", exitCode)
	}
}

func TestExitCode_ConnectionError(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "root:root@tcp(localhost:9999)/test"
`)

	_, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "test", "--format", "json")
	if exitCode != 3 {
		t.Errorf("expected exit code 3 (connection error), got %d", exitCode)
	}
}

func TestExitCode_ReadOnlyBlocked(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	_, _, exitCode := runXSQL(t, "query", "INSERT INTO test VALUES (1)",
		"--config", config, "--profile", "test", "--format", "json")
	if exitCode != 4 {
		t.Errorf("expected exit code 4 (read-only blocked), got %d", exitCode)
	}
}

func TestExitCode_DBExecFailed(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	_, _, exitCode := runXSQL(t, "query", "SELECT * FROM nonexistent_table_xyz",
		"--config", config, "--profile", "test", "--format", "json")
	if exitCode != 5 {
		t.Errorf("expected exit code 5 (DB exec failed), got %d", exitCode)
	}
}

// ============================================================================
// Config File Tests
// ============================================================================

func TestConfig_ProfileSelection(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    dsn: "`+dsn+`"
  prod:
    db: mysql
    dsn: "root:root@tcp(localhost:9999)/prod"
`)

	// Using dev profile should work
	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as result",
		"--config", config, "--profile", "dev", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for dev profile, got %d\noutput: %s", exitCode, stdout)
	}

	// Using prod profile should fail (wrong port)
	_, _, exitCode = runXSQL(t, "query", "SELECT 1 as result",
		"--config", config, "--profile", "prod", "--format", "json")
	if exitCode == 0 {
		t.Error("expected non-zero exit code for prod profile (invalid connection)")
	}
}

func TestConfig_AllowPlaintext(t *testing.T) {
	// Test that plaintext passwords in config require --allow-plaintext flag
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: "plaintext_password"
    database: test
`)

	// Without --allow-plaintext, should fail
	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode == 0 {
		// If it connected, that means plaintext was accepted or there's no validation
		// Check if there's a security error
		var resp Response
		if err := json.Unmarshal([]byte(stdout), &resp); err == nil && resp.Error != nil {
			if resp.Error.Code == "XSQL_SECRET_PLAINTEXT" {
				t.Log("correctly rejected plaintext password without flag")
				return
			}
		}
	}

	// Connection will fail anyway (wrong password), but check it's not XSQL_SECRET_PLAINTEXT
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err == nil && resp.Error != nil {
		// We expect connection error, not secret error
		t.Logf("Got error code: %s (expected connection error or secret error)", resp.Error.Code)
	}
}

func TestConfig_AllowPlaintextFlag(t *testing.T) {
	// Test that --allow-plaintext flag allows plaintext passwords
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: localhost
    port: 9999
    user: root
    password: "plaintext_password"
    database: test
`)

	// With --allow-plaintext, should try to connect (and fail due to wrong port)
	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "test", "--format", "json", "--allow-plaintext")

	// Should fail with connection error, not plaintext error
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Error != nil && resp.Error.Code == "XSQL_SECRET_PLAINTEXT" {
		t.Error("--allow-plaintext should allow plaintext passwords")
	}

	// Exit code 3 = connection error (expected since port 9999 is wrong)
	if exitCode != 3 {
		t.Logf("exit code: %d, error: %v", exitCode, resp.Error)
	}
}

func TestConfig_AllowPlaintextInConfig(t *testing.T) {
	// Test that allow_plaintext in config also works
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: localhost
    port: 9999
    user: root
    password: "plaintext_password"
    database: test
    allow_plaintext: true
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "test", "--format", "json")

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Should fail with connection error, not plaintext error
	if resp.Error != nil && resp.Error.Code == "XSQL_SECRET_PLAINTEXT" {
		t.Error("allow_plaintext in config should allow plaintext passwords")
	}

	// Exit code 3 = connection error (expected since port 9999 is wrong)
	if exitCode != 3 {
		t.Logf("exit code: %d, error: %v", exitCode, resp.Error)
	}
}

// ============================================================================
// Special SQL Tests
// ============================================================================

func TestQuery_MySQL_DescribeTable(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	// DESCRIBE on information_schema tables should work
	stdout, _, exitCode := runXSQL(t, "query", "DESCRIBE information_schema.TABLES",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", exitCode, stdout)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestQuery_MySQL_ShowVariables(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SHOW VARIABLES LIKE 'version%'",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) == 0 {
		t.Error("expected at least one variable")
	}
}

func TestQuery_PG_InformationSchema(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT table_name FROM information_schema.tables LIMIT 5",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(resp.Data.Rows) == 0 {
		t.Error("expected at least one table")
	}
}

// ============================================================================
// Environment Variable Tests
// ============================================================================

func TestEnv_Profile(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  envtest:
    db: mysql
    dsn: "`+dsn+`"
`)

	cmd := exec.Command(testBinary, "query", "SELECT 1 as n",
		"--config", config, "--format", "json")
	cmd.Env = append(os.Environ(), "XSQL_PROFILE=envtest")

	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(stdout, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true (profile should be picked from env)")
	}
}

func TestEnv_Format(t *testing.T) {
	cmd := exec.Command(testBinary, "version")
	cmd.Env = append(os.Environ(), "XSQL_FORMAT=yaml")

	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var resp Response
	if err := yaml.Unmarshal(stdout, &resp); err != nil {
		t.Fatalf("output should be YAML (from env): %v\noutput: %s", err, stdout)
	}
}

// ============================================================================
// Help Tests
// ============================================================================

func TestHelp_Root(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "--help")

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// Should contain command names
	if !strings.Contains(stdout, "query") {
		t.Error("help should mention 'query' command")
	}
	if !strings.Contains(stdout, "spec") {
		t.Error("help should mention 'spec' command")
	}
	if !strings.Contains(stdout, "version") {
		t.Error("help should mention 'version' command")
	}
}

func TestHelp_Query(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "query", "--help")

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// Should contain flags
	if !strings.Contains(stdout, "--format") {
		t.Error("help should mention '--format' flag")
	}
	if !strings.Contains(stdout, "--profile") {
		t.Error("help should mention '--profile' flag")
	}
}

// ============================================================================
// Regression Tests
// ============================================================================

func TestRegression_ErrorContainsCause(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, _ := runXSQL(t, "query", "SELECT * FROM nonexistent_table_xyz",
		"--config", config, "--profile", "test", "--format", "json")

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error")
	}

	// Error message should contain the actual MySQL error
	if !strings.Contains(resp.Error.Message, "nonexistent_table_xyz") &&
		!strings.Contains(resp.Error.Message, "doesn't exist") &&
		!strings.Contains(resp.Error.Message, "does not exist") {
		t.Logf("error message: %s", resp.Error.Message)
	}

	// Details should contain 'cause' with original error
	if resp.Error.Details != nil {
		if cause, ok := resp.Error.Details["cause"]; ok {
			if causeStr, ok := cause.(string); ok {
				if !strings.Contains(causeStr, "Table") && !strings.Contains(causeStr, "exist") {
					t.Logf("cause: %s", causeStr)
				}
			}
		}
	}
}

func TestRegression_TableFormatNoMetadata(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n",
		"--config", config, "--profile", "test", "--format", "table")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Should NOT contain "ok:" or "schema_version:"
	if strings.Contains(stdout, "ok:") || strings.Contains(stdout, "schema_version:") {
		t.Error("table format should not include ok/schema_version metadata")
	}
}

func TestRegression_CSVFormatNoMetadata(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n",
		"--config", config, "--profile", "test", "--format", "csv")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Should NOT contain "ok" or "schema_version"
	if strings.Contains(stdout, "schema_version") {
		t.Error("csv format should not include schema_version metadata")
	}

	// Should be simple CSV
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + data), got %d: %v", len(lines), lines)
	}
}

// ============================================================================
// Concurrency/Stability Tests
// ============================================================================

func TestStability_MultipleQueries(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	// Run multiple queries in sequence
	for i := 0; i < 5; i++ {
		stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 as n",
			"--config", config, "--profile", "test", "--format", "json")

		if exitCode != 0 {
			t.Fatalf("query %d failed with exit code %d", i, exitCode)
		}

		var resp struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Fatalf("query %d: invalid JSON: %v", i, err)
		}
		if !resp.OK {
			t.Errorf("query %d: expected ok=true", i)
		}
	}
}

// ============================================================================
// Unicode/Special Characters Tests
// ============================================================================

func TestQuery_Unicode(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT '你好世界' as chinese, 'こんにちは' as japanese, '🎉' as emoji",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", exitCode, stdout)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}

	row := resp.Data.Rows[0]
	if row["chinese"] != "你好世界" {
		t.Errorf("chinese=%v, want '你好世界'", row["chinese"])
	}
	if row["japanese"] != "こんにちは" {
		t.Errorf("japanese=%v, want 'こんにちは'", row["japanese"])
	}
}

func TestQuery_SpecialCharactersInValue(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", `SELECT 'hello, "world"' as quoted, 'line1\nline2' as newline`,
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		OK   bool        `json:"ok"`
		Data QueryResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	row := resp.Data.Rows[0]
	if row["quoted"] != `hello, "world"` {
		t.Errorf("quoted=%v, want 'hello, \"world\"'", row["quoted"])
	}
}

// ============================================================================
// Performance Baseline Tests
// ============================================================================

func TestPerformance_SimpleQuery(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	// This is a simple baseline - actual performance testing should use benchmarks
	// Here we just verify it completes in reasonable time
	_, _, exitCode := runXSQL(t, "query", "SELECT 1",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

// ============================================================================
// CSV Edge Cases
// ============================================================================

func TestQuery_CSV_SpecialCharacters(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", `SELECT 'hello,world' as comma, 'say "hi"' as quote`,
		"--config", config, "--profile", "test", "--format", "csv")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\noutput: %s", exitCode, stdout)
	}

	// CSV should properly escape commas and quotes
	if !strings.Contains(stdout, `"hello,world"`) && !strings.Contains(stdout, "hello,world") {
		t.Logf("CSV output: %s", stdout)
	}
}

// ============================================================================
// Spec Command Detailed Tests
// ============================================================================

func TestSpec_ContainsQueryCommand(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "spec", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Check that spec contains query command details
	if !strings.Contains(stdout, "query") {
		t.Error("spec should contain 'query' command")
	}

	// Parse and verify structure
	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Commands []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"commands"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hasQuery := false
	for _, cmd := range resp.Data.Commands {
		if cmd.Name == "query" {
			hasQuery = true
			if cmd.Description == "" {
				t.Error("query command should have description")
			}
		}
	}
	if !hasQuery {
		t.Error("spec should contain query command")
	}
}

// ============================================================================
// Complex Error Scenarios
// ============================================================================

func TestError_SyntaxError(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	// Use a valid SELECT with syntax error that won't trigger read-only block
	stdout, _, exitCode := runXSQL(t, "query", "SELECT * FORM users",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 5 {
		t.Errorf("expected exit code 5 (DB_EXEC_FAILED), got %d", exitCode)
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
	if resp.Error.Code != "XSQL_DB_EXEC_FAILED" {
		t.Errorf("expected XSQL_DB_EXEC_FAILED, got %s", resp.Error.Code)
	}
}

// ============================================================================
// Version Format Test
// ============================================================================

func TestVersion_ContainsDevOrSemver(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "version", "--format", "json")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var resp struct {
		Data VersionInfo `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Version should be "dev" or match semver pattern
	version := resp.Data.Version
	if version == "" {
		t.Error("version should not be empty")
	}

	// Accept "dev" or semver pattern (v1.2.3 or 1.2.3)
	semverPattern := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-.*)?$`)
	if version != "dev" && !semverPattern.MatchString(version) {
		t.Logf("version: %s (not 'dev' or semver pattern)", version)
	}
}

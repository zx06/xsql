//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var testBinary string

func TestMain(m *testing.M) {
	// Build test binary
	tmpDir, err := os.MkdirTemp("", "xsql-integration-test")
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

func TestCLI_Spec(t *testing.T) {
	cmd := exec.Command(testBinary, "spec", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("spec failed: %v", err)
	}

	var resp struct {
		OK            bool `json:"ok"`
		SchemaVersion int  `json:"schema_version"`
		Data          any  `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", resp.SchemaVersion)
	}
}

func TestCLI_Version(t *testing.T) {
	cmd := exec.Command(testBinary, "version", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Version == "" {
		t.Error("expected version string")
	}
}

func TestCLI_QueryWithMySQL(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	// 创建临时配置文件
	tmpDir, err := os.MkdirTemp("", "xsql-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `profiles:
  test:
    db: mysql
    dsn: "` + dsn + `"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(testBinary, "query", "SELECT 1 AS result",
		"--config", configPath,
		"--profile", "test",
		"--format", "json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("query failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Columns []string         `json:"columns"`
			Rows    []map[string]any `json:"rows"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if len(resp.Data.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(resp.Data.Rows))
	}
}

func TestCLI_QueryWithPostgres(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_PG_DSN not set")
	}

	tmpDir, err := os.MkdirTemp("", "xsql-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `profiles:
  test:
    db: pg
    dsn: "` + dsn + `"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(testBinary, "query", "SELECT 1 AS result",
		"--config", configPath,
		"--profile", "test",
		"--format", "json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("query failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Columns []string         `json:"columns"`
			Rows    []map[string]any `json:"rows"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestCLI_QueryReadOnlyBlocked(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}

	tmpDir, err := os.MkdirTemp("", "xsql-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `profiles:
  test:
    db: mysql
    dsn: "` + dsn + `"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	// 尝试执行写操作
	cmd := exec.Command(testBinary, "query", "INSERT INTO test VALUES (1)",
		"--config", configPath,
		"--profile", "test",
		"--format", "json")
	out, err := cmd.Output()
	if err == nil {
		t.Log("command should fail for write in read-only mode")
	}

	// 检查返回的错误
	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &resp); err == nil {
		if resp.OK {
			t.Error("expected ok=false for blocked write")
		}
		if resp.Error.Code != "XSQL_RO_BLOCKED" {
			t.Errorf("expected XSQL_RO_BLOCKED, got %s", resp.Error.Code)
		}
	}
}

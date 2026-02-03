package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain_SpecCommand 测试 spec 命令输出
func TestMain_SpecCommand(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "spec", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("spec command failed: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
	}

	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
	if v, _ := resp["schema_version"].(float64); v != 1 {
		t.Errorf("expected schema_version=1, got %v", v)
	}
	if resp["data"] == nil {
		t.Error("expected data field")
	}
}

// TestMain_VersionCommand 测试 version 命令
func TestMain_VersionCommand(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "version", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
	}

	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("expected ok=true")
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data map")
	}
	if _, ok := data["version"]; !ok {
		t.Error("expected version in data")
	}
}

// TestMain_QueryCommand_NoProfile 测试 query 命令（无 profile）
func TestMain_QueryCommand_NoProfile(t *testing.T) {
	binary := buildTestBinary(t)

	// 设置临时工作目录，避免读取默认配置文件
	tmpDir := t.TempDir()

	// 创建一个空的配置文件路径（确保不会读取现有配置）
	nonExistentConfig := filepath.Join(tmpDir, "nonexistent_config.yaml")

	cmd := exec.Command(binary, "query", "SELECT 1", "--format", "json", "--config", nonExistentConfig)
	cmd.Dir = tmpDir
	// 清除环境变量，避免读取配置文件
	cmd.Env = append(os.Environ(), "XSQL_CONFIG=", "HOME="+tmpDir, "USERPROFILE="+tmpDir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		t.Fatalf("expected error when no db type configured, got none. stderr: %s, stdout: %s", stderr.String(), string(out))
	}

	// 应该输出错误 JSON
	if len(out) > 0 {
		var resp map[string]any
		if err := json.Unmarshal(out, &resp); err == nil {
			if ok, _ := resp["ok"].(bool); ok {
				t.Errorf("expected ok=false, got ok=true. stderr: %s", stderr.String())
			}
		}
	}
	if stderr.Len() > 0 {
		t.Logf("stderr: %s", stderr.String())
	}
}

// TestMain_InvalidFormat 测试无效格式
func TestMain_InvalidFormat(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "spec", "--format", "invalid")
	out, _ := cmd.Output()

	// 应该有错误输出
	if len(out) == 0 {
		t.Log("no output, checking exit code")
		return
	}

	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err == nil {
		if ok, _ := resp["ok"].(bool); ok {
			t.Error("expected ok=false for invalid format")
		}
	}
}

// TestMain_Help 测试帮助
func TestMain_Help(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	if !strings.Contains(string(out), "xsql") {
		t.Errorf("expected help output to contain 'xsql', got: %s", out)
	}
}

// TestMain_ProfileListCommand 测试 profile list 命令
func TestMain_ProfileListCommand(t *testing.T) {
	binary := buildTestBinary(t)

	// 创建测试配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
profiles:
  test-db:
    description: "测试数据库"
    db: mysql
    host: localhost
    port: 3306
    user: root
    database: testdb
  prod-db:
    description: "生产环境数据库"
    db: pg
    host: prod.example.com
    port: 5432
    user: admin
    database: proddb
    unsafe_allow_write: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cmd := exec.Command(binary, "profile", "list", "--config", configPath, "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("profile list command failed: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
	}

	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data map")
	}

	profiles, ok := data["profiles"].([]any)
	if !ok {
		t.Fatal("expected profiles array")
	}

	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}

	// 验证每个 profile 的 description 字段
	expectedDescriptions := map[string]string{
		"test-db": "测试数据库",
		"prod-db": "生产环境数据库",
	}
	for _, p := range profiles {
		pm, ok := p.(map[string]any)
		if !ok {
			t.Fatal("expected profile to be a map")
		}
		name, _ := pm["name"].(string)
		expectedDesc, exists := expectedDescriptions[name]
		if !exists {
			t.Errorf("unexpected profile name: %s", name)
			continue
		}
		desc, ok := pm["description"].(string)
		if !ok || desc != expectedDesc {
			t.Errorf("profile %s: expected description=%q, got %q (ok=%v)", name, expectedDesc, desc, ok)
		}
	}
}

// TestMain_ProfileShowCommand 测试 profile show 命令
func TestMain_ProfileShowCommand(t *testing.T) {
	binary := buildTestBinary(t)

	// 创建测试配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
profiles:
  test-db:
    description: "测试数据库描述"
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: "secret"
    database: testdb
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cmd := exec.Command(binary, "profile", "show", "test-db", "--config", configPath, "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("profile show command failed: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
	}

	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data map")
	}

	// 验证 description 字段
	if desc, ok := data["description"].(string); !ok || desc != "测试数据库描述" {
		t.Errorf("expected description='测试数据库描述', got %v", data["description"])
	}

	// 验证密码被脱敏
	if pwd, ok := data["password"].(string); !ok || pwd != "***" {
		t.Errorf("expected password='***', got %v", data["password"])
	}

	// 验证其他字段
	if data["db"] != "mysql" {
		t.Errorf("expected db='mysql', got %v", data["db"])
	}
	if data["host"] != "localhost" {
		t.Errorf("expected host='localhost', got %v", data["host"])
	}
}

// TestMain_ProfileShowCommand_NotFound 测试 profile show 命令（profile 不存在）
func TestMain_ProfileShowCommand_NotFound(t *testing.T) {
	binary := buildTestBinary(t)

	// 创建测试配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
profiles:
  test-db:
    db: mysql
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cmd := exec.Command(binary, "profile", "show", "non-existent", "--config", configPath, "--format", "json")
	out, _ := cmd.Output()

	if len(out) > 0 {
		var resp map[string]any
		if err := json.Unmarshal(out, &resp); err == nil {
			if ok, _ := resp["ok"].(bool); ok {
				t.Error("expected ok=false for non-existent profile")
			}
		}
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "xsql_test_binary")
	if isWindows() {
		tmpFile += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", tmpFile, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v\n%s", err, out)
	}

	return tmpFile
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}

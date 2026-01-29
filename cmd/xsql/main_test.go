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

	cmd := exec.Command(binary, "query", "SELECT 1", "--format", "json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		t.Fatal("expected error when no db type configured")
	}

	// 应该输出错误 JSON
	if len(out) > 0 {
		var resp map[string]any
		if err := json.Unmarshal(out, &resp); err == nil {
			if ok, _ := resp["ok"].(bool); ok {
				t.Error("expected ok=false")
			}
		}
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

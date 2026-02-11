//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSchemaDump_JSON(t *testing.T) {
	config := createTempConfig(t, fmt.Sprintf(`profiles:
  dev:
    description: "开发环境"
    db: mysql
    dsn: "%s"
`, mysqlDSN(t)))
	stdout, stderr, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "json")

	// 验证退出码
	if exitCode != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", exitCode, stderr)
	}

	// 验证 JSON 格式
	var resp struct {
		OK            bool `json:"ok"`
		SchemaVersion int  `json:"schema_version"`
		Data          struct {
			Database string `json:"database"`
			Tables   []struct {
				Schema  string `json:"schema"`
				Name    string `json:"name"`
				Comment string `json:"comment"`
				Columns []struct {
					Name       string `json:"name"`
					Type       string `json:"type"`
					Nullable   bool   `json:"nullable"`
					PrimaryKey bool   `json:"primary_key"`
				} `json:"columns"`
				Indexes []struct {
					Name    string   `json:"name"`
					Columns []string `json:"columns"`
					Unique  bool     `json:"unique"`
					Primary bool     `json:"primary"`
				} `json:"indexes"`
				ForeignKeys []struct {
					Name              string   `json:"name"`
					Columns           []string `json:"columns"`
					ReferencedTable   string   `json:"referenced_table"`
					ReferencedColumns []string `json:"referenced_columns"`
				} `json:"foreign_keys"`
			} `json:"tables"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Errorf("failed to parse JSON output: %v, stdout: %s", err, stdout)
		return
	}

	// 验证 schema_version
	if resp.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", resp.SchemaVersion)
	}

	// 如果成功，验证数据结构
	if resp.OK {
		if resp.Data.Database == "" {
			t.Error("database name is empty")
		}
		// tables 可以为空（数据库无表）
	}
}

func TestSchemaDump_YAML(t *testing.T) {
	config := createTempConfig(t, fmt.Sprintf(`profiles:
  dev:
    description: "开发环境"
    db: pg
    dsn: "%s"
`, pgDSN(t)))
	stdout, stderr, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "yaml")

	// 验证退出码
	if exitCode != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", exitCode, stderr)
	}

	// 验证 YAML 格式包含必要字段
	if !contains(stdout, "ok:") {
		t.Error("YAML output missing 'ok:' field")
	}
	if !contains(stdout, "schema_version:") {
		t.Error("YAML output missing 'schema_version:' field")
	}
}

func TestSchemaDump_Table(t *testing.T) {
	config := createTempConfig(t, fmt.Sprintf(`profiles:
  dev:
    description: "开发环境"
    db: mysql
    dsn: "%s"
`, mysqlDSN(t)))
	stdout, stderr, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "table")

	// 验证退出码
	if exitCode != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", exitCode, stderr)
	}

	// 验证 Table 格式不包含 JSON 元数据
	if contains(stdout, `"ok"`) {
		t.Error("Table output should not contain JSON 'ok' field")
	}
	if contains(stdout, `"schema_version"`) {
		t.Error("Table output should not contain JSON 'schema_version' field")
	}
}

func TestSchemaDump_ProfileNotFound(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: 127.0.0.1
`)
	stdout, _, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "nonexistent", "-f", "json")

	// 验证退出码（配置错误）
	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	// 验证错误响应
	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Errorf("failed to parse JSON: %v", err)
		return
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error.Code == "" {
		t.Error("error code is empty")
	}
}

func TestSchemaDump_MissingProfile(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: 127.0.0.1
`)
	_, _, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-f", "json")

	// 验证退出码（参数错误）
	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}
}

func TestSchemaDump_TableFilter(t *testing.T) {
	config := createTempConfig(t, fmt.Sprintf(`profiles:
  dev:
    description: "开发环境"
    db: mysql
    dsn: "%s"
`, mysqlDSN(t)))
	// 使用 --table 过滤
	stdout, stderr, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "json", "--table", "user*")

	// 验证退出码
	if exitCode != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", exitCode, stderr)
	}

	// 如果成功，验证过滤生效
	if exitCode == 0 {
		var resp struct {
			OK   bool `json:"ok"`
			Data struct {
				Tables []struct {
					Name string `json:"name"`
				} `json:"tables"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Errorf("failed to parse JSON: %v", err)
			return
		}
		// 所有表名应该以 user 开头
		for _, table := range resp.Data.Tables {
			if len(table.Name) < 4 || table.Name[:4] != "user" {
				t.Errorf("table name %q does not match filter 'user*'", table.Name)
			}
		}
	}
}

func TestSchemaDump_Help(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "schema", "dump", "--help")

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}

	// 验证帮助信息包含关键内容
	if !contains(stdout, "schema dump") {
		t.Error("help output missing 'schema dump'")
	}
	if !contains(stdout, "--table") {
		t.Error("help output missing '--table' flag")
	}
	if !contains(stdout, "--include-system") {
		t.Error("help output missing '--include-system' flag")
	}
}

func TestSchema_Command(t *testing.T) {
	// 测试 schema 父命令
	stdout, _, exitCode := runXSQL(t, "schema", "--help")

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}

	if !contains(stdout, "dump") {
		t.Error("schema command help should mention 'dump' subcommand")
	}
}

func TestSchemaDump_MissingDBType(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    host: 127.0.0.1
`)
	stdout, _, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "json")

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error.Code == "" {
		t.Error("error code is empty")
	}
}

func TestSchemaDump_PlaintextPasswordNotAllowed(t *testing.T) {
	config := createTempConfig(t, fmt.Sprintf(`profiles:
  dev:
    description: "开发环境"
    db: mysql
    dsn: "%s"
    password: "plain_password"
`, mysqlDSN(t)))

	stdout, _, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "json")

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error.Code == "" {
		t.Error("error code is empty")
	}
}

func TestSchemaDump_InvalidFormat(t *testing.T) {
	config := createTempConfig(t, fmt.Sprintf(`profiles:
  dev:
    description: "开发环境"
    db: mysql
    dsn: "%s"
`, mysqlDSN(t)))

	stdout, _, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "invalid")

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error.Code == "" {
		t.Error("error code is empty")
	}
}

func TestSchemaDump_UnsupportedDriver(t *testing.T) {
	config := createTempConfig(t, fmt.Sprintf(`profiles:
  dev:
    description: "开发环境"
    db: sqlite
    dsn: "%s"
`, mysqlDSN(t)))

	stdout, _, exitCode := runXSQL(t, "schema", "dump", "--config", config, "-p", "dev", "-f", "json")

	if exitCode == 0 {
		t.Errorf("exit code = %d, want non-zero", exitCode)
	}

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Error.Code == "" {
		t.Error("error code is empty")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPServer_Startup tests that MCP server starts correctly and returns tools list
func TestMCPServer_Startup(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: test123
    database: testdb
    allow_plaintext: true
  prod:
    db: pg
    host: prod.example.com
    port: 5432
    user: readonly
    password: secret
    database: proddb
    allow_plaintext: true
`)

	tools := listMCPTools(t, config)

	// Verify all expected tools are present
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{"query", "profile_list", "profile_show"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

// TestMCPServer_EnumValidation tests that profile names are validated against enum values
func TestMCPServer_EnumValidation(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: test123
    database: testdb
    allow_plaintext: true
  prod:
    db: pg
    host: prod.example.com
    port: 5432
    user: readonly
    password: secret
    database: proddb
    allow_plaintext: true
`)

	tools := listMCPTools(t, config)

	// Find profile_show tool and check its schema
	var profileShowTool *mcp.Tool
	for i := range tools {
		if tools[i].Name == "profile_show" {
			profileShowTool = &tools[i]
			break
		}
	}

	if profileShowTool == nil {
		t.Fatal("profile_show tool not found")
	}

	// Check that input schema has enum with both profiles
	schemaJSON, err := json.Marshal(profileShowTool.InputSchema)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		t.Fatalf("failed to unmarshal schema: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties not found")
	}

	nameProp, ok := properties["name"].(map[string]any)
	if !ok {
		t.Fatal("name property not found in schema")
	}

	enum, ok := nameProp["enum"].([]any)
	if !ok {
		t.Fatal("enum not found in name property")
	}

	// Check both profiles are in enum
	hasDev := false
	hasProd := false
	for _, v := range enum {
		if v == "dev" {
			hasDev = true
		}
		if v == "prod" {
			hasProd = true
		}
	}

	if !hasDev {
		t.Error("enum should contain 'dev'")
	}
	if !hasProd {
		t.Error("enum should contain 'prod'")
	}
}

// TestMCPServer_QueryToolEnum tests that query tool has profile enum
func TestMCPServer_QueryToolEnum(t *testing.T) {
	config := createTempConfig(t, `profiles:
  mysql-dev:
    db: mysql
    host: localhost
    user: root
    password: test
    database: testdb
    allow_plaintext: true
  pg-prod:
    db: pg
    host: prod.example.com
    user: readonly
    password: secret
    database: proddb
    allow_plaintext: true
`)

	tools := listMCPTools(t, config)

	// Find query tool and check its schema
	var queryTool *mcp.Tool
	for i := range tools {
		if tools[i].Name == "query" {
			queryTool = &tools[i]
			break
		}
	}

	if queryTool == nil {
		t.Fatal("query tool not found")
	}

	// Check that input schema has enum with both profiles
	schemaJSON, err := json.Marshal(queryTool.InputSchema)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		t.Fatalf("failed to unmarshal schema: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties not found")
	}

	profileProp, ok := properties["profile"].(map[string]any)
	if !ok {
		t.Fatal("profile property not found in schema")
	}

	enum, ok := profileProp["enum"].([]any)
	if !ok {
		t.Fatal("enum not found in profile property")
	}

	// Check both profiles are in enum
	hasMysqlDev := false
	hasPgProd := false
	for _, v := range enum {
		if v == "mysql-dev" {
			hasMysqlDev = true
		}
		if v == "pg-prod" {
			hasPgProd = true
		}
	}

	if !hasMysqlDev {
		t.Error("enum should contain 'mysql-dev'")
	}
	if !hasPgProd {
		t.Error("enum should contain 'pg-prod'")
	}
}

// TestMCPServer_EmptyConfig tests MCP server with empty config
func TestMCPServer_EmptyConfig(t *testing.T) {
	config := createTempConfig(t, `profiles: {}`)

	tools := listMCPTools(t, config)

	// Verify tools are still present even with empty config
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	// Tools should still exist, but enums will be empty
	if !toolNames["profile_show"] {
		t.Error("profile_show tool should exist even with empty config")
	}

	if !toolNames["query"] {
		t.Error("query tool should exist even with empty config")
	}
}

// listMCPTools lists all available MCP tools
func listMCPTools(t *testing.T, configPath string) []mcp.Tool {
	t.Helper()

	// Build the test binary
	tmpDir := t.TempDir()
	binary := tmpDir + "/xsql"
	if testBinary != "" {
		binary = testBinary
	}

	cmd := exec.Command(binary, "mcp", "server", "--config", configPath)

	// Start the server
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout: %v", err)
	}

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start MCP server: %v", err)
	}

	// Send initialize request
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	initJSON, _ := json.Marshal(initReq)
	stdin.Write(initJSON)
	stdin.Write([]byte("\n"))

	// Read initialize response
	buf := make([]byte, 4096)
	n, _ := stdout.Read(buf)

	// Send tools/list request
	listReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	}

	listJSON, _ := json.Marshal(listReq)
	stdin.Write(listJSON)
	stdin.Write([]byte("\n"))

	// Read tools/list response
	n, _ = stdout.Read(buf)

	// Parse response
	var response struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}

	// Find the JSON object in the response
	start := bytes.Index(buf[:n], []byte("{"))
	if start == -1 {
		t.Fatalf("no JSON object found in response")
	}

	if err := json.Unmarshal(buf[start:n], &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nresponse: %s", err, string(buf[start:n]))
	}

	// Kill the server
	cmd.Process.Kill()
	cmd.Wait()

	return response.Result.Tools
}

// callMCPTool calls an MCP tool and returns the result
func callMCPTool(t *testing.T, configPath string, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	t.Helper()

	binary := testBinary
	if binary == "" {
		t.Fatal("testBinary is not set")
	}

	cmd := exec.Command(binary, "mcp", "server", "--config", configPath)

	// Start the server
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout: %v", err)
	}

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start MCP server: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Send initialize request
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	initJSON, _ := json.Marshal(initReq)
	stdin.Write(initJSON)
	stdin.Write([]byte("\n"))

	// Read initialize response
	buf := make([]byte, 8192)
	stdout.Read(buf)

	// Send tools/call request
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	callJSON, _ := json.Marshal(callReq)
	stdin.Write(callJSON)
	stdin.Write([]byte("\n"))

	// Read tools/call response
	n, _ := stdout.Read(buf)

	// Parse response
	var response struct {
		Result mcp.CallToolResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	// Find the JSON object in the response
	start := bytes.Index(buf[:n], []byte("{"))
	if start == -1 {
		t.Fatalf("no JSON object found in response")
	}

	if err := json.Unmarshal(buf[start:n], &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nresponse: %s", err, string(buf[start:n]))
	}

	if response.Error != nil {
		return nil, &mcpError{code: response.Error.Code, message: response.Error.Message}
	}

	return &response.Result, nil
}

type mcpError struct {
	code    int
	message string
}

func (e *mcpError) Error() string {
	return e.message
}

// ============================================================================
// MCP Query E2E Tests
// ============================================================================

// TestMCPQuery_MySQL_BasicSelect tests executing a basic SELECT via MCP
func TestMCPQuery_MySQL_BasicSelect(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1 as num, 'hello' as msg",
		"profile": "test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	// Parse the JSON response
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Columns []string         `json:"columns"`
			Rows    []map[string]any `json:"rows"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(textContent.Text), &response); err != nil {
		t.Fatalf("failed to parse result: %v\ntext: %s", err, textContent.Text)
	}

	if !response.OK {
		t.Error("expected ok=true")
	}

	if len(response.Data.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(response.Data.Columns))
	}

	if len(response.Data.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(response.Data.Rows))
	}

	if response.Data.Rows[0]["msg"] != "hello" {
		t.Errorf("expected msg='hello', got %v", response.Data.Rows[0]["msg"])
	}
}

// TestMCPQuery_MySQL_MultipleRows tests query returning multiple rows
func TestMCPQuery_MySQL_MultipleRows(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1 as n UNION SELECT 2 UNION SELECT 3",
		"profile": "test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		Data struct {
			Rows []map[string]any `json:"rows"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if len(response.Data.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(response.Data.Rows))
	}
}

// TestMCPQuery_MySQL_ReadOnlyBlocked tests that write operations are blocked
func TestMCPQuery_MySQL_ReadOnlyBlocked(t *testing.T) {
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
		{"CREATE", "CREATE TABLE test (id INT)"},
		{"DROP", "DROP TABLE test"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := callMCPTool(t, config, "query", map[string]any{
				"sql":     tc.sql,
				"profile": "test",
			})

			if err != nil {
				t.Fatalf("MCP query failed: %v", err)
			}

			// Should return error result
			if !result.IsError {
				t.Error("expected IsError=true for write operation")
			}

			textContent := result.Content[0].(*mcp.TextContent)
			var response struct {
				OK    bool `json:"ok"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			json.Unmarshal([]byte(textContent.Text), &response)

			if response.OK {
				t.Error("expected ok=false")
			}

			if response.Error.Code != "XSQL_RO_BLOCKED" {
				t.Errorf("expected XSQL_RO_BLOCKED, got %s", response.Error.Code)
			}
		})
	}
}

// TestMCPQuery_MySQL_InvalidSQL tests error handling for invalid SQL
func TestMCPQuery_MySQL_InvalidSQL(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT * FROM nonexistent_table_xyz",
		"profile": "test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true for invalid SQL")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	if response.Error.Code != "XSQL_DB_EXEC_FAILED" {
		t.Errorf("expected XSQL_DB_EXEC_FAILED, got %s", response.Error.Code)
	}
}

// TestMCPQuery_MissingSQL tests error when SQL is missing
func TestMCPQuery_MissingSQL(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "root:root@tcp(localhost:3306)/test"
    allow_plaintext: true
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"profile": "test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true for missing SQL")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	if response.Error.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected XSQL_CFG_INVALID, got %s", response.Error.Code)
	}
}

// TestMCPQuery_MissingProfile tests error when profile is missing
func TestMCPQuery_MissingProfile(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "root:root@tcp(localhost:3306)/test"
    allow_plaintext: true
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql": "SELECT 1",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true for missing profile")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	if response.Error.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected XSQL_CFG_INVALID, got %s", response.Error.Code)
	}
}

// TestMCPQuery_ProfileNotFound tests error when profile doesn't exist
func TestMCPQuery_ProfileNotFound(t *testing.T) {
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "root:root@tcp(localhost:3306)/test"
    allow_plaintext: true
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1",
		"profile": "nonexistent",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true for nonexistent profile")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	if response.Error.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected XSQL_CFG_INVALID, got %s", response.Error.Code)
	}

	if response.Error.Details != nil {
		if reason, ok := response.Error.Details["reason"]; ok {
			if reason != "profile_not_found" {
				t.Errorf("expected reason=profile_not_found, got %v", reason)
			}
		}
	}
}

// TestMCPQuery_PG_BasicSelect tests PostgreSQL query via MCP
func TestMCPQuery_PG_BasicSelect(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1 as num, 'hello' as msg",
		"profile": "test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Columns []string         `json:"columns"`
			Rows    []map[string]any `json:"rows"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if !response.OK {
		t.Error("expected ok=true")
	}

	if len(response.Data.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(response.Data.Columns))
	}

	if len(response.Data.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(response.Data.Rows))
	}
}

// TestMCPQuery_DataTypes tests various data types via MCP
func TestMCPQuery_DataTypes(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 42 as int_val, 3.14 as float_val, 'text' as str_val, TRUE as bool_val, NULL as null_val",
		"profile": "test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		Data struct {
			Rows []map[string]any `json:"rows"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	row := response.Data.Rows[0]
	if row["str_val"] != "text" {
		t.Errorf("str_val=%v, want 'text'", row["str_val"])
	}

	if row["null_val"] != nil {
		t.Errorf("null_val=%v, want nil", row["null_val"])
	}
}

// TestMCPProfileList tests profile_list tool
func TestMCPProfileList(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: localhost
    description: "Dev database"
  prod:
    db: pg
    host: prod.example.com
    description: "Prod database"
    unsafe_allow_write: true
`)

	result, err := callMCPTool(t, config, "profile_list", map[string]any{})

	if err != nil {
		t.Fatalf("MCP profile_list failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Profiles []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				DB          string `json:"db"`
				Mode        string `json:"mode"`
			} `json:"profiles"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if !response.OK {
		t.Error("expected ok=true")
	}

	if len(response.Data.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(response.Data.Profiles))
	}

	// Find prod profile and check mode
	for _, p := range response.Data.Profiles {
		if p.Name == "prod" {
			if p.Mode != "read-write" {
				t.Errorf("expected prod mode=read-write, got %s", p.Mode)
			}
		}
		if p.Name == "dev" {
			if p.Mode != "read-only" {
				t.Errorf("expected dev mode=read-only, got %s", p.Mode)
			}
		}
	}
}

// TestMCPProfileShow tests profile_show tool
func TestMCPProfileShow(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: secret123
    database: testdb
    description: "Dev database"
    allow_plaintext: true
`)

	result, err := callMCPTool(t, config, "profile_show", map[string]any{
		"name": "dev",
	})

	if err != nil {
		t.Fatalf("MCP profile_show failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Name        string `json:"name"`
			DB          string `json:"db"`
			Host        string `json:"host"`
			Port        int    `json:"port"`
			User        string `json:"user"`
			Password    string `json:"password"`
			Database    string `json:"database"`
			Description string `json:"description"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if !response.OK {
		t.Error("expected ok=true")
	}

	if response.Data.Name != "dev" {
		t.Errorf("expected name=dev, got %s", response.Data.Name)
	}

	if response.Data.DB != "mysql" {
		t.Errorf("expected db=mysql, got %s", response.Data.DB)
	}

	// Password should be redacted
	if response.Data.Password != "***" {
		t.Errorf("expected password=***, got %s", response.Data.Password)
	}

	// Verify password is not exposed
	if bytes.Contains([]byte(textContent.Text), []byte("secret123")) {
		t.Error("password should not be exposed in profile_show")
	}
}

// TestMCPProfileShow_NotFound tests profile_show with non-existent profile
func TestMCPProfileShow_NotFound(t *testing.T) {
	config := createTempConfig(t, `profiles:
  dev:
    db: mysql
    host: localhost
`)

	result, err := callMCPTool(t, config, "profile_show", map[string]any{
		"name": "nonexistent",
	})

	if err != nil {
		t.Fatalf("MCP profile_show failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true for nonexistent profile")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	if response.Error.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected XSQL_CFG_INVALID, got %s", response.Error.Code)
	}
}

// ============================================================================
// MCP SSH Proxy Tests
// ============================================================================

// TestMCPQuery_SSHProxy_ConfigurationHandling tests SSH proxy configuration
func TestMCPQuery_SSHProxy_ConfigurationHandling(t *testing.T) {
	dsn := mysqlDSN(t)

	// Note: This test verifies SSH proxy configuration is properly processed,
	// but will fail at SSH connection stage since no real SSH server exists.
	// This is expected and allows us to verify configuration handling.
	config := createTempConfig(t, `ssh_proxies:
  bastion:
    host: nonexistent.ssh.server.test
    port: 22
    user: testuser
    identity_file: ~/.ssh/id_rsa
    skip_host_key: true

profiles:
  ssh-test:
    db: mysql
    dsn: "`+dsn+`"
    ssh_proxy: bastion
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1",
		"profile": "ssh-test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	// Should return error due to SSH connection failure
	if !result.IsError {
		t.Error("expected IsError=true for SSH connection failure")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	// Should fail with SSH dial error
	if response.Error.Code != "XSQL_SSH_DIAL_FAILED" {
		t.Logf("Got error code: %s (expected SSH_DIAL_FAILED or SSH_AUTH_FAILED)", response.Error.Code)
		// Either SSH_DIAL_FAILED or SSH_AUTH_FAILED is acceptable depending on timing
		if response.Error.Code != "XSQL_SSH_AUTH_FAILED" {
			t.Errorf("expected XSQL_SSH_DIAL_FAILED or XSQL_SSH_AUTH_FAILED, got %s", response.Error.Code)
		}
	}
}

// TestMCPQuery_SSHProxy_InvalidConfiguration tests SSH proxy errors
func TestMCPQuery_SSHProxy_InvalidConfiguration(t *testing.T) {
	config := createTempConfig(t, `profiles:
  ssh-test:
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: test
    database: testdb
    ssh_proxy: nonexistent_proxy
    allow_plaintext: true
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1",
		"profile": "ssh-test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	// Should return error for non-existent SSH proxy
	if !result.IsError {
		t.Error("expected IsError=true for non-existent SSH proxy")
	}

	textContent2 := result.Content[0].(*mcp.TextContent)
	var response2 struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent2.Text), &response2)

	if response2.OK {
		t.Error("expected ok=false")
	}

	// Should fail with configuration error
	if response2.Error.Code != "XSQL_CFG_INVALID" {
		t.Errorf("expected XSQL_CFG_INVALID, got %s", response2.Error.Code)
	}

	// Error message should mention the proxy
	if !bytes.Contains([]byte(response2.Error.Message), []byte("proxy")) &&
		!bytes.Contains([]byte(response2.Error.Message), []byte("nonexistent_proxy")) {
		t.Logf("Error message: %s (should mention proxy issue)", response2.Error.Message)
	}
}

// TestMCPQuery_SSHProxy_MissingIdentityFile tests SSH key file errors
func TestMCPQuery_SSHProxy_MissingIdentityFile(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `ssh_proxies:
  bastion:
    host: localhost
    port: 22
    user: testuser
    identity_file: /nonexistent/path/to/key
    skip_host_key: true

profiles:
  ssh-test:
    db: mysql
    dsn: "`+dsn+`"
    ssh_proxy: bastion
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1",
		"profile": "ssh-test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	// Should return error for missing identity file
	if !result.IsError {
		t.Error("expected IsError=true for missing identity file")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	// Should fail with CFG_INVALID or SSH_AUTH_FAILED
	if response.Error.Code != "XSQL_CFG_INVALID" && response.Error.Code != "XSQL_SSH_AUTH_FAILED" {
		t.Errorf("expected XSQL_CFG_INVALID or XSQL_SSH_AUTH_FAILED, got %s", response.Error.Code)
	}
}

// TestMCPProfileShow_WithSSHProxy tests SSH proxy info in profile_show
func TestMCPProfileShow_WithSSHProxy(t *testing.T) {
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

	result, err := callMCPTool(t, config, "profile_show", map[string]any{
		"name": "prod",
	})

	if err != nil {
		t.Fatalf("MCP profile_show failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Name            string `json:"name"`
			DB              string `json:"db"`
			Host            string `json:"host"`
			SSHProxy        string `json:"ssh_proxy"`
			SSHHost         string `json:"ssh_host"`
			SSHPort         int    `json:"ssh_port"`
			SSHUser         string `json:"ssh_user"`
			SSHIdentityFile string `json:"ssh_identity_file"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if !response.OK {
		t.Error("expected ok=true")
	}

	// Verify SSH proxy information is included
	if response.Data.SSHProxy != "bastion" {
		t.Errorf("expected ssh_proxy=bastion, got %s", response.Data.SSHProxy)
	}

	if response.Data.SSHHost != "bastion.example.com" {
		t.Errorf("expected ssh_host=bastion.example.com, got %s", response.Data.SSHHost)
	}

	if response.Data.SSHPort != 22 {
		t.Errorf("expected ssh_port=22, got %d", response.Data.SSHPort)
	}

	if response.Data.SSHUser != "admin" {
		t.Errorf("expected ssh_user=admin, got %s", response.Data.SSHUser)
	}

	if response.Data.SSHIdentityFile != "~/.ssh/id_rsa" {
		t.Errorf("expected ssh_identity_file=~/.ssh/id_rsa, got %s", response.Data.SSHIdentityFile)
	}
}

// TestMCPProfileList_WithSSHProxy tests SSH proxy profiles in profile_list
func TestMCPProfileList_WithSSHProxy(t *testing.T) {
	config := createTempConfig(t, `ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin

profiles:
  direct:
    db: mysql
    host: localhost
    description: "Direct connection"
  tunneled:
    db: pg
    host: db.internal
    description: "SSH tunneled connection"
    ssh_proxy: bastion
`)

	result, err := callMCPTool(t, config, "profile_list", map[string]any{})

	if err != nil {
		t.Fatalf("MCP profile_list failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %v", result.Content)
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Profiles []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				DB          string `json:"db"`
				Mode        string `json:"mode"`
			} `json:"profiles"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if !response.OK {
		t.Error("expected ok=true")
	}

	// Should list both profiles
	if len(response.Data.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(response.Data.Profiles))
	}

	// Verify both direct and tunneled profiles exist
	hasDirect := false
	hasTunneled := false
	for _, p := range response.Data.Profiles {
		if p.Name == "direct" {
			hasDirect = true
		}
		if p.Name == "tunneled" {
			hasTunneled = true
		}
	}

	if !hasDirect {
		t.Error("expected 'direct' profile in list")
	}
	if !hasTunneled {
		t.Error("expected 'tunneled' profile in list")
	}
}

// TestMCPQuery_SSHProxy_PassphraseHandling tests SSH key passphrase scenarios
func TestMCPQuery_SSHProxy_PassphraseHandling(t *testing.T) {
	dsn := mysqlDSN(t)

	// Test with invalid passphrase format (should fail at secret resolution)
	config := createTempConfig(t, `ssh_proxies:
  bastion:
    host: localhost
    port: 22
    user: testuser
    identity_file: ~/.ssh/id_rsa
    passphrase: "keyring:invalid:format:too:many:parts"
    skip_host_key: true

profiles:
  ssh-test:
    db: mysql
    dsn: "`+dsn+`"
    ssh_proxy: bastion
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1",
		"profile": "ssh-test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	// Should return error for invalid passphrase format
	if !result.IsError {
		t.Error("expected IsError=true for invalid passphrase format")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	// Should fail with secret error or SSH auth error
	if response.Error.Code != "XSQL_SECRET_INVALID" &&
		response.Error.Code != "XSQL_SSH_AUTH_FAILED" &&
		response.Error.Code != "XSQL_CFG_INVALID" {
		t.Logf("Got error code: %s (expected XSQL_SECRET_INVALID, XSQL_SSH_AUTH_FAILED, or XSQL_CFG_INVALID)", response.Error.Code)
	}
}

// TestMCPQuery_SSHProxy_HostKeyValidation tests SSH host key scenarios
func TestMCPQuery_SSHProxy_HostKeyValidation(t *testing.T) {
	dsn := mysqlDSN(t)

	// Test without skip_host_key (should fail with known_hosts error)
	config := createTempConfig(t, `ssh_proxies:
  bastion:
    host: unknown.host.test
    port: 22
    user: testuser
    identity_file: ~/.ssh/id_rsa
    skip_host_key: false

profiles:
  ssh-test:
    db: mysql
    dsn: "`+dsn+`"
    ssh_proxy: bastion
`)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1",
		"profile": "ssh-test",
	})

	if err != nil {
		t.Fatalf("MCP query failed: %v", err)
	}

	// Should return error
	if !result.IsError {
		t.Error("expected IsError=true for host key validation")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	var response struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal([]byte(textContent.Text), &response)

	if response.OK {
		t.Error("expected ok=false")
	}

	// Could fail with SSH_HOST_KEY_MISMATCH or other SSH errors
	t.Logf("Error code: %s", response.Error.Code)
}

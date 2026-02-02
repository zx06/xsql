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

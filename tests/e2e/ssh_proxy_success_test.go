//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ============================================================================
// MCP SSH Proxy Success Tests (requires real SSH server)
// ============================================================================

// getSSHTestConfig returns SSH test configuration from environment variables
func getSSHTestConfig() (host, port, user, keyPath, knownHosts string, enabled bool) {
	host = os.Getenv("SSH_TEST_HOST")
	port = os.Getenv("SSH_TEST_PORT")
	user = os.Getenv("SSH_TEST_USER")
	keyPath = os.Getenv("SSH_TEST_KEY_PATH")
	knownHosts = os.Getenv("SSH_KNOWN_HOSTS_FILE")
	enabled = host != "" && port != "" && user != "" && keyPath != ""
	return
}

// TestMCPQuery_SSHProxy_Success_MySQL tests successful MySQL query via SSH proxy
func TestMCPQuery_SSHProxy_Success_MySQL(t *testing.T) {
	host, port, user, keyPath, knownHosts, enabled := getSSHTestConfig()
	if !enabled {
		t.Skip("SSH test environment not configured (set SSH_TEST_HOST, SSH_TEST_PORT, SSH_TEST_USER, SSH_TEST_KEY_PATH)")
	}

	// Use MySQL container IP from environment if available (for Docker network access via SSH)
	mysqlIP := os.Getenv("MYSQL_IP")
	if mysqlIP == "" {
		mysqlIP = "127.0.0.1"
	}
	dsn := fmt.Sprintf("root:root@tcp(%s:3306)/testdb", mysqlIP)

	// Build SSH proxy configuration
	sshProxyConfig := fmt.Sprintf(`ssh_proxies:
  test-bastion:
    host: %s
    port: %s
    user: %s
    identity_file: %s
`, host, port, user, keyPath)

	// Add known_hosts if available (for strict host key checking)
	if knownHosts != "" {
		sshProxyConfig = fmt.Sprintf(`ssh_proxies:
  test-bastion:
    host: %s
    port: %s
    user: %s
    identity_file: %s
    known_hosts_file: %s
`, host, port, user, keyPath, knownHosts)
	} else {
		// Skip host key check if known_hosts not provided
		sshProxyConfig = fmt.Sprintf(`ssh_proxies:
  test-bastion:
    host: %s
    port: %s
    user: %s
    identity_file: %s
    skip_host_key: true
`, host, port, user, keyPath)
	}

	configContent := sshProxyConfig + `
profiles:
  ssh-mysql:
    db: mysql
    dsn: "` + dsn + `"
    ssh_proxy: test-bastion
`
	config := createTempConfig(t, configContent)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1 as test_val",
		"profile": "ssh-mysql",
	})

	if err != nil {
		t.Fatalf("MCP query via SSH proxy failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Query should succeed via SSH proxy, got error: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent := result.Content[0].(*mcp.TextContent)
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

	if len(response.Data.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(response.Data.Rows))
	}

	t.Log("MySQL query via SSH proxy succeeded")
}

// TestMCPQuery_SSHProxy_Success_PG tests successful PostgreSQL query via SSH proxy
func TestMCPQuery_SSHProxy_Success_PG(t *testing.T) {
	host, port, user, keyPath, knownHosts, enabled := getSSHTestConfig()
	if !enabled {
		t.Skip("SSH test environment not configured (set SSH_TEST_HOST, SSH_TEST_PORT, SSH_TEST_USER, SSH_TEST_KEY_PATH)")
	}

	// Use PostgreSQL container IP from environment if available (for Docker network access via SSH)
	pgIP := os.Getenv("PG_IP")
	if pgIP == "" {
		pgIP = "127.0.0.1"
	}
	dsn := fmt.Sprintf("postgres://postgres:postgres@%s:5432/testdb?sslmode=disable", pgIP)

	// Build SSH proxy configuration
	var sshProxyConfig string
	if knownHosts != "" {
		sshProxyConfig = fmt.Sprintf(`ssh_proxies:
  test-bastion:
    host: %s
    port: %s
    user: %s
    identity_file: %s
    known_hosts_file: %s
`, host, port, user, keyPath, knownHosts)
	} else {
		sshProxyConfig = fmt.Sprintf(`ssh_proxies:
  test-bastion:
    host: %s
    port: %s
    user: %s
    identity_file: %s
    skip_host_key: true
`, host, port, user, keyPath)
	}

	configContent := sshProxyConfig + `
profiles:
  ssh-pg:
    db: pg
    dsn: "` + dsn + `"
    ssh_proxy: test-bastion
`
	config := createTempConfig(t, configContent)

	result, err := callMCPTool(t, config, "query", map[string]any{
		"sql":     "SELECT 1 as test_val",
		"profile": "ssh-pg",
	})

	if err != nil {
		t.Fatalf("MCP query via SSH proxy failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Query should succeed via SSH proxy, got error: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent := result.Content[0].(*mcp.TextContent)
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

	if len(response.Data.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(response.Data.Rows))
	}

	t.Log("PostgreSQL query via SSH proxy succeeded")
}

// TestMCPQuery_SSHProxy_MultipleQueries tests multiple queries through same SSH tunnel
func TestMCPQuery_SSHProxy_MultipleQueries(t *testing.T) {
	host, port, user, keyPath, knownHosts, enabled := getSSHTestConfig()
	if !enabled {
		t.Skip("SSH test environment not configured")
	}

	// Use MySQL container IP from environment if available
	mysqlIP := os.Getenv("MYSQL_IP")
	if mysqlIP == "" {
		mysqlIP = "127.0.0.1"
	}
	dsn := fmt.Sprintf("root:root@tcp(%s:3306)/testdb", mysqlIP)

	var sshProxyConfig string
	if knownHosts != "" {
		sshProxyConfig = fmt.Sprintf(`ssh_proxies:
  test-bastion:
    host: %s
    port: %s
    user: %s
    identity_file: %s
    known_hosts_file: %s
`, host, port, user, keyPath, knownHosts)
	} else {
		sshProxyConfig = fmt.Sprintf(`ssh_proxies:
  test-bastion:
    host: %s
    port: %s
    user: %s
    identity_file: %s
    skip_host_key: true
`, host, port, user, keyPath)
	}

	configContent := sshProxyConfig + `
profiles:
  ssh-mysql:
    db: mysql
    dsn: "` + dsn + `"
    ssh_proxy: test-bastion
`
	config := createTempConfig(t, configContent)

	// Run multiple queries
	queries := []string{
		"SELECT 1 as val",
		"SELECT 2 as val",
		"SELECT 'hello' as msg",
	}

	for i, sql := range queries {
		result, err := callMCPTool(t, config, "query", map[string]any{
			"sql":     sql,
			"profile": "ssh-mysql",
		})

		if err != nil {
			t.Fatalf("Query %d failed: %v", i+1, err)
		}

		if result.IsError {
			t.Fatalf("Query %d should succeed, got error: %v", i+1, result.Content)
		}
	}

	t.Log("Multiple queries via SSH proxy succeeded")
}

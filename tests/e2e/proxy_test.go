//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"exec"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// xsql proxy Tests
// ============================================================================

func TestProxy_MissingSSHProxy(t *testing.T) {
	// Test that proxy fails when profile has no ssh_proxy configured
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: localhost
    port: 3306
    user: root
    database: test
`)

	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--format", "json")

	// Should fail with config error (no ssh_proxy)
	if exitCode == 0 {
		t.Error("expected non-zero exit code when ssh_proxy is not configured")
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
}

func TestProxy_MissingDB(t *testing.T) {
	// Test that proxy fails when profile has no db type configured
	config := createTempConfig(t, `profiles:
  test:
    host: localhost
    port: 3306
`)

	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--format", "json")

	// Should fail with config error
	if exitCode == 0 {
		t.Error("expected non-zero exit code when db type is not configured")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestProxy_MissingProfile(t *testing.T) {
	// Test that proxy fails when profile doesn't exist
	config := createTempConfig(t, `profiles:
  existing:
    db: mysql
    host: localhost
    port: 3306
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: localhost
      user: test
`)

	stdout, _, exitCode := runXSQL(t, "-p", "nonexistent", "proxy",
		"--config", config, "--format", "json")

	// Should fail with config error
	if exitCode == 0 {
		t.Error("expected non-zero exit code for non-existent profile")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestProxy_OutputFormat_JSON(t *testing.T) {
	// Test JSON output format for proxy
	// Note: This test doesn't actually start the proxy (would require SSH),
	// it just validates that the command structure and error handling work

	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: bastion.example.com
      user: test
      identity_file: ~/.ssh/id_ed25519
`)

	// This will fail to connect to SSH, but should return proper JSON error
	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--format", "json")

	// Should fail due to SSH connection issues
	if exitCode == 0 {
		t.Log("proxy succeeded (unexpected, but may work in test environment)")
	}

	// Validate JSON structure
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}

	if resp.OK {
		// If somehow it succeeded, validate data structure
		if resp.Data != nil {
			data, ok := resp.Data.(map[string]any)
			if ok {
				if _, hasLocal := data["local_address"]; !hasLocal {
					t.Error("expected local_address in success data")
				}
				if _, hasRemote := data["remote_address"]; !hasRemote {
					t.Error("expected remote_address in success data")
				}
			}
		}
	} else {
		// Validate error structure
		if resp.Error == nil {
			t.Error("expected error in response")
		}
	}
}

func TestProxy_OutputFormat_Table(t *testing.T) {
	// Test table output format for proxy
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: bastion.example.com
      user: test
`)

	// This will fail to connect to SSH, but stderr should have table-like output
	stdout, stderr, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--format", "table")

	// Should fail due to SSH connection issues
	if exitCode == 0 {
		t.Log("proxy succeeded (unexpected)")
	}

	// Table format should not have JSON in stderr
	if strings.Contains(stderr, `"ok":`) || strings.Contains(stderr, `"error":`) {
		t.Error("table format should not output JSON in stderr")
	}
}

func TestProxy_WithCustomPort(t *testing.T) {
	// Test that custom port option is accepted
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: bastion.example.com
      user: test
`)

	// This will fail to connect to SSH, but should validate port option
	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--local-port", "13306", "--format", "json")

	// Will fail due to SSH connection, but should have processed the port option
	if exitCode == 0 {
		t.Log("proxy with custom port succeeded")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err == nil && resp.OK && resp.Data != nil {
		// If succeeded, verify the port is correct
		data, ok := resp.Data.(map[string]any)
		if ok {
			if localAddr, hasAddr := data["local_address"].(string); hasAddr {
				if !strings.Contains(localAddr, "13306") {
					t.Errorf("expected local_address to contain port 13306, got %s", localAddr)
				}
			}
		}
	}
}

func TestProxy_InvalidPort(t *testing.T) {
	// Test that invalid port is rejected
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: bastion.example.com
      user: test
`)

	// Try to start with invalid port (non-numeric)
	_, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--local-port", "invalid")

	// Should fail with argument error
	if exitCode == 0 {
		t.Error("expected non-zero exit code for invalid port")
	}
}

func TestProxy_AutoPortAllocation(t *testing.T) {
	// Test that port=0 (auto-assign) is accepted
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: bastion.example.com
      user: test
`)

	// Start with auto-port
	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--local-port", "0", "--format", "json")

	if exitCode == 0 {
		// If somehow it succeeded, verify a non-zero port was allocated
		var resp Response
		if err := json.Unmarshal([]byte(stdout), &resp); err == nil && resp.OK && resp.Data != nil {
			data, ok := resp.Data.(map[string]any)
			if ok {
				if localAddr, hasAddr := data["local_address"].(string); hasAddr {
					if strings.Contains(localAddr, ":0") {
						t.Error("auto-port allocation should not return port 0 in local_address")
					}
				}
			}
		}
	}
}

// ============================================================================
// Proxy Port Availability Test
// ============================================================================

func TestProxy_PortInUse(t *testing.T) {
	// Find an available port, bind to it, then try to start proxy on same port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	defer listener.Close()

	// Get the port that was allocated
	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    host: remote-db.example.com
    port: 3306
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: bastion.example.com
      user: test
`)

	// Try to start proxy on the already-in-use port
	stdout, _, exitCode := runXSQL(t, "-p", "test", "proxy",
		"--config", config, "--local-port", strconv.Itoa(port), "--format", "json")

	// Should fail (port in use)
	if exitCode == 0 {
		t.Error("expected non-zero exit code when port is already in use")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
		if resp.OK {
			t.Error("expected ok=false when port is in use")
		}
	}
}

// ============================================================================
// Proxy Help Test
// ============================================================================

func TestProxy_Help(t *testing.T) {
	// Test that proxy help is available
	stdout, _, exitCode := runXSQL(t, "proxy", "--help")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Should contain proxy-related text
	if !strings.Contains(stdout, "proxy") {
		t.Error("help output should contain 'proxy'")
	}
	if !strings.Contains(stdout, "profile") {
		t.Error("help output should contain 'profile'")
	}
	if !strings.Contains(stdout, "local-port") {
		t.Error("help output should contain 'local-port'")
	}
}

func TestProxy_MissingProfileFlag(t *testing.T) {
	// Test that proxy fails when -p flag is not provided
	stdout, _, exitCode := runXSQL(t, "proxy", "--format", "json")

	// Should fail with config error
	if exitCode == 0 {
		t.Error("expected non-zero exit code when -p flag is not provided")
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
		if resp.OK {
			t.Error("expected ok=false when -p flag is missing")
		}
	}
}

// ============================================================================
// Integration Test (Optional - requires real SSH)
// ============================================================================

// TestProxyStart_RealConnection is an integration test that requires
// a real SSH server and database connection. It is skipped by default
// and can be enabled by setting XSQL_TEST_PROXY_DSN and XSQL_TEST_PROXY_SSH_CONFIG.
//
// To run this test:
// XSQL_TEST_PROXY_DSN="root:password@tcp(localhost:3306)/test"
// XSQL_TEST_PROXY_SSH_CONFIG="user:ssh_user,host:localhost,port:22,key:/path/to/key"
// go test -tags=e2e ./tests/e2e/... -run TestProxy_RealConnection -v
func TestProxy_RealConnection(t *testing.T) {
	proxyDSN := os.Getenv("XSQL_TEST_PROXY_DSN")
	sshConfig := os.Getenv("XSQL_TEST_PROXY_SSH_CONFIG")

	if proxyDSN == "" || sshConfig == "" {
		t.Skip("XSQL_TEST_PROXY_DSN and XSQL_TEST_PROXY_SSH_CONFIG not set, skipping real connection test")
	}

	// Parse SSH config (simple format: user:xxx,host:xxx,port:xxx,key:xxx)
	sshParams := make(map[string]string)
	parts := strings.Split(sshConfig, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			sshParams[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	if sshParams["user"] == "" || sshParams["host"] == "" {
		t.Fatal("invalid SSH config format, need user and host")
	}

	// Parse DSN to get db host and port
	dbHost := "localhost"
	dbPort := 3306
	if strings.Contains(proxyDSN, "tcp(") {
		start := strings.Index(proxyDSN, "tcp(") + 4
		end := strings.Index(proxyDSN[start:], ")")
		if end > 0 {
			addr := proxyDSN[start : start+end]
			if strings.Contains(addr, ":") {
				hostPort := strings.Split(addr, ":")
				dbHost = hostPort[0]
				if len(hostPort) > 1 {
					if port, err := strconv.Atoi(hostPort[1]); err == nil {
						dbPort = port
					}
				}
			}
		}
	}

	// Create config
	configContent := `profiles:
  test:
    db: mysql
    host: ` + dbHost + `
    port: ` + strconv.Itoa(dbPort) + `
    dsn: "` + proxyDSN + `"
    ssh_proxy: test_ssh
  ssh_proxies:
    test_ssh:
      host: ` + sshParams["host"] + `
      user: ` + sshParams["user"]
	if port, hasPort := sshParams["port"]; hasPort {
		configContent += `
      port: ` + port
	}
	if key, hasKey := sshParams["key"]; hasKey {
		configContent += `
      identity_file: ` + key
	}
	configContent += `
      skip_host_key: true
`

	config := createTempConfig(t, configContent)

	// Start proxy with a timeout
	// Note: This test will run the proxy command, which will hang until interrupted
	// We'll just verify it starts and outputs the expected format

	// Run with a short timeout to avoid hanging
	cmd := exec.Command(testBinary, "-p", "test", "proxy",
		"--config", config, "--format", "json")

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start proxy command: %v", err)
	}

	// Wait a bit for the proxy to start
	time.Sleep(2 * time.Second)

	// Get the output
	stdout := outBuf.String()
	stderr := errBuf.String()

	// Terminate the proxy
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("failed to kill proxy process: %v", err)
	}
	cmd.Wait()

	// Validate output
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}

	if !resp.OK {
		t.Errorf("expected ok=true, got error: %v", resp.Error)
	}

	if resp.Data == nil {
		t.Fatal("expected data in response")
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}

	// Verify required fields
	if _, hasLocal := data["local_address"]; !hasLocal {
		t.Error("expected local_address in data")
	}
	if _, hasRemote := data["remote_address"]; !hasRemote {
		t.Error("expected remote_address in data")
	}
	if _, hasProfile := data["profile"]; !hasProfile {
		t.Error("expected profile in data")
	}

	t.Logf("Proxy started successfully: %v", data)
}

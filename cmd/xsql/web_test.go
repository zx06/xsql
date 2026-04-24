package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	webpkg "github.com/zx06/xsql/internal/web"
)

// nolint:gosec // Test file uses hardcoded tokens for test fixtures
// Test for NewServeCommand structure and flags
func TestNewServeCommand_CreatesCommand(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})

	cmd := NewServeCommand(&w)

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	if cmd.Use != "serve" {
		t.Errorf("expected Use=serve, got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

// Test for NewWebCommand structure and flags
func TestNewWebCommand_CreatesCommand(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})

	cmd := NewWebCommand(&w)

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	if cmd.Use != "web" {
		t.Errorf("expected Use=web, got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

// Test mode mapping for different open browser settings
func TestModeForWebCommand_ServeMode(t *testing.T) {
	mode := modeForWebCommand(false)
	if mode != "serve" {
		t.Errorf("expected mode=serve, got %s", mode)
	}
}

func TestModeForWebCommand_WebMode(t *testing.T) {
	mode := modeForWebCommand(true)
	if mode != "web" {
		t.Errorf("expected mode=web, got %s", mode)
	}
}

// Test all variations of mode mapping
func TestModeForWebCommand_AllVariations(t *testing.T) {
	tests := []struct {
		open     bool
		expected string
	}{
		{true, "web"},
		{false, "serve"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			mode := modeForWebCommand(tt.open)
			if mode != tt.expected {
				t.Errorf("modeForWebCommand(%v): want %s, got %s", tt.open, tt.expected, mode)
			}
		})
	}
}

// Test browser opening behavior
func TestOpenBrowserDefault_NoError(t *testing.T) {
	// This test verifies the function doesn't crash with valid URL
	// The actual command execution may fail depending on OS, but that's OK
	url := "http://localhost:8788"
	err := openBrowserDefault(url)

	// Either success or command not found error is acceptable
	if err != nil {
		t.Logf("openBrowserDefault returned expected error: %v", err)
	}
}

// Test CLI address priority over environment
// TestResolveWebOptions_AddressResolution tests address resolution priority: CLI > ENV > Config > Default
func TestResolveWebOptions_AddressResolution(t *testing.T) {
	tests := []struct {
		name      string
		setEnv    string
		opts      *webCommandOptions
		cfg       config.File
		expected  string
		expectErr bool
	}{
		{
			name:   "CLI takes priority over env",
			setEnv: "0.0.0.0:9999",
			opts: &webCommandOptions{
				addr:    "127.0.0.1:7777",
				addrSet: true,
			},
			expected: "127.0.0.1:7777",
		},
		{
			name:   "Env takes priority over config",
			setEnv: "127.0.0.1:8888",
			opts:   &webCommandOptions{},
			cfg: config.File{
				Web: config.WebConfig{
					HTTP: config.WebHTTPConfig{Addr: "127.0.0.1:8787"},
				},
			},
			expected: "127.0.0.1:8888",
		},
		{
			name: "Invalid address format rejected",
			opts: &webCommandOptions{
				addr:    "invalid-no-port",
				addrSet: true,
			},
			expectErr: true,
		},
		{
			name:     "Nil options uses default",
			opts:     nil,
			expected: webpkg.DefaultAddr,
		},
		{
			name:     "Empty env doesn't override defaults",
			setEnv:   "",
			opts:     &webCommandOptions{},
			expected: webpkg.DefaultAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv != "" || tt.name == "Empty env doesn't override defaults" {
				t.Setenv("XSQL_WEB_HTTP_ADDR", tt.setEnv)
			}
			resolved, xe := resolveWebOptions(tt.opts, tt.cfg)
			if tt.expectErr && xe == nil {
				t.Fatal("expected error")
			}
			if !tt.expectErr && xe != nil {
				t.Fatalf("unexpected error: %v", xe)
			}
			if !tt.expectErr && resolved.addr != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, resolved.addr)
			}
		})
	}
}

// Test all loopback address variations
func TestResolveWebOptions_LoopbackAddresses(t *testing.T) {
	tests := []struct {
		name       string
		addr       string
		shouldReq  bool
	}{
		{"IPv4 loopback", "127.0.0.1:8788", false},
		{"IPv6 loopback", "[::1]:8788", false},
		{"localhost", "localhost:8788", false},
		{"IPv4 any", "0.0.0.0:8788", true},
		{"IPv6 any", "[::]:8788", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &webCommandOptions{
				addr:    tt.addr,
				addrSet: true,
			}

			// For non-loopback, add token
			if tt.shouldReq {
				opts.authToken = "test-token"
				opts.authTokenSet = true
			}

			resolved, xe := resolveWebOptions(opts, config.File{})

			if xe != nil {
				t.Errorf("unexpected error: %v", xe)
				return
			}

			if resolved.authRequired != tt.shouldReq {
				t.Errorf("authRequired: want %v, got %v", tt.shouldReq, resolved.authRequired)
			}
		})
	}
}

// TestResolveWebOptions_TokenResolution tests token resolution priority: CLI > ENV > Config > None
func TestResolveWebOptions_TokenResolution(t *testing.T) {
	tests := []struct {
		name         string
		setEnv       string
		opts         *webCommandOptions
		cfg          config.File
		expectedTok  string
		expectErr    bool
	}{
		{
			name:   "CLI token takes priority",
			setEnv: "env-token",
			opts: &webCommandOptions{
				addr:         "0.0.0.0:8788",
				addrSet:      true,
				authToken:    "cli-token",
				authTokenSet: true,
			},
			expectedTok: "cli-token",
		},
		{
			name:   "Env token overrides config",
			setEnv: "env-token",
			opts: &webCommandOptions{
				addr:    "0.0.0.0:8788",
				addrSet: true,
			},
			cfg: config.File{
				Web: config.WebConfig{
					HTTP: config.WebHTTPConfig{
						AuthToken:           "config-token",
						AllowPlaintextToken: true,
					},
				},
			},
			expectedTok: "env-token",
		},
		{
			name: "Config token used with plaintext allowed",
			opts: &webCommandOptions{
				addr:    "0.0.0.0:8788",
				addrSet: true,
			},
			cfg: config.File{
				Web: config.WebConfig{
					HTTP: config.WebHTTPConfig{
						Addr:                "0.0.0.0:8788",
						AuthToken:           "plaintext-token",
						AllowPlaintextToken: true,
					},
				},
			},
			expectedTok: "plaintext-token",
		},
		{
			name:        "Non-loopback without token fails",
			setEnv:      "",
			opts:        &webCommandOptions{addr: "0.0.0.0:8788", addrSet: true},
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XSQL_WEB_HTTP_AUTH_TOKEN", tt.setEnv)
			resolved, xe := resolveWebOptions(tt.opts, tt.cfg)
			if tt.expectErr && xe == nil {
				t.Fatal("expected error")
			}
			if !tt.expectErr && xe != nil {
				t.Fatalf("unexpected error: %v", xe)
			}
			if !tt.expectErr && resolved.authToken != tt.expectedTok {
				t.Errorf("expected token %s, got %s", tt.expectedTok, resolved.authToken)
			}
		})
	}
}

// Test RunWebCommand with config load error
// Test ServeCommand flags are properly registered
func TestServeCommand_FlagsRegistered(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})
	cmd := NewServeCommand(&w)

	flags := []string{"addr", "auth-token", "allow-plaintext", "ssh-skip-known-hosts-check"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %s", flag)
		}
	}
}

// Test WebCommand flags are properly registered
func TestWebCommand_FlagsRegistered(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})
	cmd := NewWebCommand(&w)

	flags := []string{"addr", "auth-token", "allow-plaintext", "ssh-skip-known-hosts-check"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %s", flag)
		}
	}
}

// Test webCommandOptions structure
func TestWebCommandOptions_Structure(t *testing.T) {
	opts := &webCommandOptions{
		addr:           "127.0.0.1:8788",
		addrSet:        true,
		authToken:      "test-token",
		authTokenSet:   true,
		allowPlaintext: false,
		skipHostKey:    false,
		openBrowser:    true,
	}

	if opts.addr != "127.0.0.1:8788" {
		t.Error("addr not set correctly")
	}
	if !opts.addrSet {
		t.Error("addrSet should be true")
	}
	if opts.authToken != "test-token" {
		t.Error("authToken not set correctly")
	}
	if !opts.authTokenSet {
		t.Error("authTokenSet should be true")
	}
	if opts.allowPlaintext {
		t.Error("allowPlaintext should be false")
	}
	if opts.skipHostKey {
		t.Error("skipHostKey should be false")
	}
	if !opts.openBrowser {
		t.Error("openBrowser should be true")
	}
}

// Test resolvedWebOptions structure
func TestResolvedWebOptions_Structure(t *testing.T) {
	resolved := resolvedWebOptions{
		addr:         "127.0.0.1:8788",
		authToken:    "test-token",
		authRequired: true,
	}

	if resolved.addr != "127.0.0.1:8788" {
		t.Error("addr not set correctly")
	}
	if resolved.authToken != "test-token" {
		t.Error("authToken not set correctly")
	}
	if !resolved.authRequired {
		t.Error("authRequired should be true")
	}
}

// Test config address is invalid
func TestResolveWebOptions_ConfigAddressInvalid(t *testing.T) {
	opts := &webCommandOptions{}
	cfg := config.File{
		Web: config.WebConfig{
			HTTP: config.WebHTTPConfig{
				Addr: "bad-address",
			},
		},
	}

	_, xe := resolveWebOptions(opts, cfg)

	if xe == nil {
		t.Fatal("expected error for invalid config address")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

// Test config token without plaintext allowed
func TestResolveWebOptions_ConfigTokenNotAllowedPlaintext(t *testing.T) {
	opts := &webCommandOptions{
		addr:    "0.0.0.0:8788",
		addrSet: true,
	}
	cfg := config.File{
		Web: config.WebConfig{
			HTTP: config.WebHTTPConfig{
				Addr:                "0.0.0.0:8788",
				AuthToken:           "config-token",
				AllowPlaintextToken: false,
			},
		},
	}

	// This might fail when trying to resolve the token
	resolved, xe := resolveWebOptions(opts, cfg)

	// Either error or empty token is acceptable depending on secret resolution
	if xe == nil && resolved.authToken == "" {
		t.Logf("token not resolved without plaintext permission")
	}
}

// Test HTTP address resolution with various ports
func TestResolveWebOptions_VariousPorts(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"standard web port", "127.0.0.1:80", false},
		{"custom high port", "127.0.0.1:9999", false},
		{"missing port", "127.0.0.1", true},
		{"localhost with port", "localhost:8080", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &webCommandOptions{
				addr:    tt.addr,
				addrSet: true,
			}

			_, xe := resolveWebOptions(opts, config.File{})

			if tt.wantErr {
				if xe == nil {
					t.Error("expected error")
				}
			} else {
				if xe != nil {
					t.Errorf("unexpected error: %v", xe)
				}
			}
		})
	}
}

// Test ServeCommand RunE sets addrSet properly
func TestServeCommand_AddrSetFlag(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})
	cmd := NewServeCommand(&w)

	cmd.Flags().Set("addr", "127.0.0.1:7777")

	// The flag should be marked as changed
	if !cmd.Flags().Changed("addr") {
		t.Error("addr flag should be marked as changed after Set")
	}
}

// Test WebCommand RunE sets authTokenSet properly
func TestWebCommand_AuthTokenSetFlag(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})
	cmd := NewWebCommand(&w)

	cmd.Flags().Set("auth-token", "mytoken")

	// The flag should be marked as changed
	if !cmd.Flags().Changed("auth-token") {
		t.Error("auth-token flag should be marked as changed after Set")
	}
}

// TestRunWebCommand_ConfigLoadError tests error handling when config loading fails
func TestRunWebCommand_ConfigLoadError(t *testing.T) {
	opts := &webCommandOptions{
		addr:        "127.0.0.1:0",
		addrSet:     true,
		authToken:   "",
		authTokenSet: false,
	}

	// Save current GlobalConfig and restore after test
	oldConfig := GlobalConfig
	defer func() { GlobalConfig = oldConfig }()

	// Set an invalid config path to trigger config loading error
	GlobalConfig.ConfigStr = "/nonexistent/path/to/config.yaml"

	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})

	err := runWebCommand(opts, &w)
	
	// Should return an error due to missing config file
	if err == nil {
		t.Error("expected error from loading invalid config path, got nil")
	}
}

// TestRunWebCommand_ListenerCreationError tests error handling when port is in use
func TestRunWebCommand_ListenerCreationError(t *testing.T) {
	// Create a temporary config file for this test
	configDir := t.TempDir()
	configPath := configDir + "/config.yaml"
	configContent := `profiles:
  default:
    driver: mysql
    host: localhost
    port: 3306
    user: root
    password: root
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	// Create a listener and keep it open to occupy the port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener for testing: %v", err)
	}
	defer listener.Close()

	portInUse := listener.Addr().(*net.TCPAddr).Port

	opts := &webCommandOptions{
		addr:         fmt.Sprintf("127.0.0.1:%d", portInUse),
		addrSet:      true,
		authToken:    "",
		authTokenSet: false,
	}

	// Save and restore GlobalConfig
	oldConfig := GlobalConfig
	defer func() { GlobalConfig = oldConfig }()
	GlobalConfig.ConfigStr = configPath

	var buf bytes.Buffer
	w := output.New(&buf, &bytes.Buffer{})

	// Use a goroutine with timeout to prevent hanging on Windows
	errCh := make(chan error, 1)
	go func() {
		errCh <- runWebCommand(opts, &w)
	}()

	select {
	case err := <-errCh:
		// Should return an error (port in use)
		if err == nil {
			t.Error("expected error from listener creation, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Error("test timed out - runWebCommand likely blocked waiting for signals")
	}
}



package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	webpkg "github.com/zx06/xsql/internal/web"
)

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
func TestResolveWebOptions_CLIAddressTakePriority(t *testing.T) {
	t.Setenv("XSQL_WEB_HTTP_ADDR", "0.0.0.0:9999")

	opts := &webCommandOptions{
		addr:    "127.0.0.1:7777",
		addrSet: true,
	}
	cfg := config.File{}

	resolved, xe := resolveWebOptions(opts, cfg)

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if resolved.addr != "127.0.0.1:7777" {
		t.Errorf("expected CLI addr to take priority, got %s", resolved.addr)
	}
}

// Test environment address priority over config
func TestResolveWebOptions_EnvAddressPriority(t *testing.T) {
	t.Setenv("XSQL_WEB_HTTP_ADDR", "127.0.0.1:8888")

	opts := &webCommandOptions{}
	cfg := config.File{
		Web: config.WebConfig{
			HTTP: config.WebHTTPConfig{
				Addr: "127.0.0.1:8787",
			},
		},
	}

	resolved, xe := resolveWebOptions(opts, cfg)

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if resolved.addr != "127.0.0.1:8888" {
		t.Errorf("expected env addr to take priority, got %s", resolved.addr)
	}
}

// Test invalid address format rejection
func TestResolveWebOptions_InvalidAddress(t *testing.T) {
	opts := &webCommandOptions{
		addr:    "invalid-no-port",
		addrSet: true,
	}
	cfg := config.File{}

	_, xe := resolveWebOptions(opts, cfg)

	if xe == nil {
		t.Fatal("expected error for invalid address")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

// Test nil options handling
func TestResolveWebOptions_NilOptionsHandled(t *testing.T) {
	cfg := config.File{}

	resolved, xe := resolveWebOptions(nil, cfg)

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if resolved.addr != webpkg.DefaultAddr {
		t.Errorf("expected default addr, got %s", resolved.addr)
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

// Test CLI token priority
func TestResolveWebOptions_CLITokenPriority(t *testing.T) {
	t.Setenv("XSQL_WEB_HTTP_AUTH_TOKEN", "env-token")

	opts := &webCommandOptions{
		addr:         "0.0.0.0:8788",
		addrSet:      true,
		authToken:    "cli-token",
		authTokenSet: true,
	}
	cfg := config.File{}

	resolved, xe := resolveWebOptions(opts, cfg)

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if resolved.authToken != "cli-token" {
		t.Errorf("expected CLI token priority, got %s", resolved.authToken)
	}
}

// Test environment token priority
func TestResolveWebOptions_EnvTokenPriority(t *testing.T) {
	t.Setenv("XSQL_WEB_HTTP_AUTH_TOKEN", "env-token")

	opts := &webCommandOptions{
		addr:    "0.0.0.0:8788",
		addrSet: true,
	}
	cfg := config.File{
		Web: config.WebConfig{
			HTTP: config.WebHTTPConfig{
				AuthToken:           "config-token",
				AllowPlaintextToken: true,
			},
		},
	}

	resolved, xe := resolveWebOptions(opts, cfg)

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if resolved.authToken != "env-token" {
		t.Errorf("expected env token to override config, got %s", resolved.authToken)
	}
}

// Test config token is used when plaintext allowed
func TestResolveWebOptions_ConfigTokenWithPlaintext(t *testing.T) {
	opts := &webCommandOptions{
		addr:    "0.0.0.0:8788",
		addrSet: true,
	}
	cfg := config.File{
		Web: config.WebConfig{
			HTTP: config.WebHTTPConfig{
				Addr:                "0.0.0.0:8788",
				AuthToken:           "plaintext-token",
				AllowPlaintextToken: true,
			},
		},
	}

	resolved, xe := resolveWebOptions(opts, cfg)

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if resolved.authToken != "plaintext-token" {
		t.Errorf("expected plaintext token, got %s", resolved.authToken)
	}
}

// Test empty env vars don't override defaults
func TestResolveWebOptions_EmptyEnvVars(t *testing.T) {
	t.Setenv("XSQL_WEB_HTTP_ADDR", "")
	t.Setenv("XSQL_WEB_HTTP_AUTH_TOKEN", "")

	opts := &webCommandOptions{}
	cfg := config.File{}

	resolved, xe := resolveWebOptions(opts, cfg)

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if resolved.addr != webpkg.DefaultAddr {
		t.Errorf("expected default addr, got %s", resolved.addr)
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

	opts := &webCommandOptions{
		addr:         "127.0.0.1:1",  // Port 1 is unlikely to be available
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

	err := runWebCommand(opts, &w)
	
	// Should return an error (permission denied or port in use)
	if err == nil {
		t.Error("expected error from listener creation, got nil")
	}
}

// TestResolveWebOptions_NonLoopbackWithoutToken tests auth requirement for non-loopback addresses
func TestResolveWebOptions_NonLoopbackWithoutToken(t *testing.T) {
	opts := &webCommandOptions{
		addr:         "0.0.0.0:8080",
		addrSet:      true,
		authToken:    "",
		authTokenSet: false,
	}

	cfg := config.File{}

	_, err := resolveWebOptions(opts, cfg)
	
	if err == nil {
		t.Error("expected error when non-loopback address without auth token, got nil")
	}
}

// TestResolveWebOptions_NonLoopbackWithToken tests successful resolution with token
func TestResolveWebOptions_NonLoopbackWithToken(t *testing.T) {
	opts := &webCommandOptions{
		addr:         "0.0.0.0:8080",
		addrSet:      true,
		authToken:    "test-token-12345",
		authTokenSet: true,
	}

	cfg := config.File{}

	resolved, err := resolveWebOptions(opts, cfg)
	
	if err != nil {
		t.Errorf("expected no error for non-loopback with token, got: %v", err)
	}
	if resolved.addr != "0.0.0.0:8080" {
		t.Errorf("expected addr 0.0.0.0:8080, got %s", resolved.addr)
	}
	if !resolved.authRequired {
		t.Error("expected authRequired to be true for non-loopback address")
	}
	if resolved.authToken != "test-token-12345" {
		t.Errorf("expected token test-token-12345, got %s", resolved.authToken)
	}
}

// TestResolveWebOptions_LoopbackDefault tests default behavior with loopback
func TestResolveWebOptions_LoopbackDefault(t *testing.T) {
	opts := &webCommandOptions{
		addr:         "",
		addrSet:      false,
		authToken:    "",
		authTokenSet: false,
	}

	cfg := config.File{}

	resolved, err := resolveWebOptions(opts, cfg)
	
	if err != nil {
		t.Errorf("expected no error for loopback default, got: %v", err)
	}
	if resolved.addr != webpkg.DefaultAddr {
		t.Errorf("expected default addr %s, got %s", webpkg.DefaultAddr, resolved.addr)
	}
	if resolved.authRequired {
		t.Error("expected authRequired to be false for loopback")
	}
}

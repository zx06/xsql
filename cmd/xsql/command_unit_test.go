package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

func TestParseOutputFormat(t *testing.T) {
	format, err := parseOutputFormat("auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != output.FormatJSON && format != output.FormatTable {
		t.Fatalf("unexpected format: %s", format)
	}

	if _, err := parseOutputFormat("invalid"); err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestResolveFormatForError(t *testing.T) {
	format := resolveFormatForError("invalid")
	if format != output.FormatJSON && format != output.FormatTable {
		t.Fatalf("unexpected format: %s", format)
	}
}

func TestNormalizeErr(t *testing.T) {
	xe := errors.New(errors.CodeCfgInvalid, "bad config", nil)
	if got := normalizeErr(xe); got != xe {
		t.Fatalf("expected same error, got %v", got)
	}

	err := normalizeErr(os.ErrInvalid)
	if err.Code != errors.CodeInternal {
		t.Fatalf("expected CodeInternal, got %s", err.Code)
	}
}

func TestResolveWebOptions_DefaultLoopback(t *testing.T) {
	resolved, xe := resolveWebOptions(&webCommandOptions{}, config.File{})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.addr != "127.0.0.1:8788" {
		t.Fatalf("addr=%q", resolved.addr)
	}
	if resolved.authRequired {
		t.Fatal("loopback address should not require auth")
	}
}

func TestResolveWebOptions_RemoteRequiresToken(t *testing.T) {
	_, xe := resolveWebOptions(&webCommandOptions{
		addr:    "0.0.0.0:8788",
		addrSet: true,
	}, config.File{})
	if xe == nil {
		t.Fatal("expected error")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("code=%s", xe.Code)
	}
}

func TestResolveWebOptions_ConfigToken(t *testing.T) {
	resolved, xe := resolveWebOptions(&webCommandOptions{
		addr:    "0.0.0.0:8788",
		addrSet: true,
	}, config.File{
		Web: config.WebConfig{
			HTTP: config.WebHTTPConfig{
				AuthToken:           "token",
				AllowPlaintextToken: true,
			},
		},
	})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if !resolved.authRequired {
		t.Fatal("expected authRequired=true")
	}
	if resolved.authToken != "token" {
		t.Fatalf("authToken=%q", resolved.authToken)
	}
}

func TestRun_SpecCommandSuccess(t *testing.T) {
	prev := GlobalConfig
	GlobalConfig = &Config{}
	t.Cleanup(func() { GlobalConfig = prev })

	prevArgs := os.Args
	os.Args = []string{"xsql", "spec", "--format", "json"}
	t.Cleanup(func() { os.Args = prevArgs })

	exitCode := run()
	if exitCode != int(errors.ExitOK) {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
}

func TestRun_InvalidFormatExitCode(t *testing.T) {
	prev := GlobalConfig
	GlobalConfig = &Config{}
	t.Cleanup(func() { GlobalConfig = prev })

	prevArgs := os.Args
	os.Args = []string{"xsql", "spec", "--format", "invalid"}
	t.Cleanup(func() { os.Args = prevArgs })

	exitCode := run()
	if exitCode != int(errors.ExitConfig) {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
}

func TestRunQuery_MissingDB(t *testing.T) {
	GlobalConfig.Resolved.Profile = configProfile("")
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runQuery([]string{"select 1"}, &QueryFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for missing db type")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunQuery_UnsupportedDriver(t *testing.T) {
	GlobalConfig.Resolved.Profile = configProfile("sqlite")
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runQuery([]string{"select 1"}, &QueryFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for unsupported driver")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeDBDriverUnsupported {
		t.Fatalf("expected CodeDBDriverUnsupported, got %v", err)
	}
}

func TestRunQuery_PlaintextPasswordNotAllowed(t *testing.T) {
	GlobalConfig.Resolved.Profile = config.Profile{
		DB:             "mysql",
		Password:       "plain_password",
		AllowPlaintext: false,
	}
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runQuery([]string{"select 1"}, &QueryFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for plaintext password not allowed")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunSchemaDump_UnsupportedDriver(t *testing.T) {
	GlobalConfig.Resolved.Profile = configProfile("sqlite")
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runSchemaDump(&SchemaFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for unsupported driver")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeDBDriverUnsupported {
		t.Fatalf("expected CodeDBDriverUnsupported, got %v", err)
	}
}

func TestRunSchemaDump_PlaintextPasswordNotAllowed(t *testing.T) {
	GlobalConfig.Resolved.Profile = config.Profile{
		DB:             "mysql",
		Password:       "plain_password",
		AllowPlaintext: false,
	}
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runSchemaDump(&SchemaFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for plaintext password not allowed")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunQuery_InvalidFormat(t *testing.T) {
	GlobalConfig.Resolved.Profile = configProfile("mysql")
	GlobalConfig.FormatStr = "invalid"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runQuery([]string{"select 1"}, &QueryFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunSchemaDump_MissingDB(t *testing.T) {
	GlobalConfig.Resolved.Profile = configProfile("")
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runSchemaDump(&SchemaFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for missing db type")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunSchemaDump_InvalidFormat(t *testing.T) {
	GlobalConfig.Resolved.Profile = configProfile("mysql")
	GlobalConfig.FormatStr = "invalid"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runSchemaDump(&SchemaFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunProxy_ProfileRequired(t *testing.T) {
	GlobalConfig.ProfileStr = ""
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunProxy_MissingDB(t *testing.T) {
	GlobalConfig.ProfileStr = "dev"
	GlobalConfig.FormatStr = "json"
	GlobalConfig.Resolved.Profile = config.Profile{}

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for missing db")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunProxy_MissingSSHConfig(t *testing.T) {
	GlobalConfig.ProfileStr = "dev"
	GlobalConfig.FormatStr = "json"
	GlobalConfig.Resolved.Profile = config.Profile{DB: "mysql"}

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for missing ssh config")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestRunProxy_PassphraseResolveError(t *testing.T) {
	GlobalConfig.ProfileStr = "dev"
	GlobalConfig.FormatStr = "json"
	GlobalConfig.Resolved.Profile = config.Profile{
		DB: "mysql",
		SSHConfig: &config.SSHProxy{
			Host:       "example.com",
			Port:       22,
			User:       "user",
			Passphrase: "keyring:missing/passphrase",
		},
	}

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for passphrase resolve")
	}
}

func TestRunProxy_SSHConnectError(t *testing.T) {
	GlobalConfig.ProfileStr = "dev"
	GlobalConfig.FormatStr = "json"
	GlobalConfig.Resolved.Profile = config.Profile{
		DB: "mysql",
		SSHConfig: &config.SSHProxy{
			Host: "",
			Port: 22,
			User: "user",
		},
	}

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for ssh connect")
	}
	if xe, ok := errors.As(err); !ok || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %v", err)
	}
}

func TestResolveSSH_NoConfig(t *testing.T) {
	client, err := app.ResolveSSH(context.TODO(), config.Profile{}, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Fatal("expected nil client")
	}
}

func TestResolveSSH_PassphraseResolveError(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host:       "example.com",
			Port:       22,
			User:       "user",
			Passphrase: "keyring:missing/passphrase",
		},
	}

	_, err := app.ResolveSSH(context.Background(), profile, false, false)
	if err == nil {
		t.Fatal("expected error for passphrase resolve")
	}
}

func TestProfileCommands_ListAndShow(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
profiles:
  dev:
    description: "Dev database"
    db: mysql
    host: localhost
    port: 3306
    user: root
    database: testdb
    password: secret
  prod:
    description: "Prod database"
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

	GlobalConfig.ConfigStr = configPath
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	listCmd := newProfileListCommand(&w)
	listCmd.SetArgs([]string{})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("expected json output, got: %s", out.String())
	}

	out.Reset()
	showCmd := newProfileShowCommand(&w)
	showCmd.SetArgs([]string{"dev"})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("show command failed: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("expected json output, got: %s", out.String())
	}
}

func TestRunMCPServer_ConfigMissing(t *testing.T) {
	GlobalConfig.ConfigStr = filepath.Join(t.TempDir(), "missing.yaml")
	err := runMCPServer(&mcpServerOptions{})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestRunMCPServer_StdioTreatsContextCanceledAsCleanExit(t *testing.T) {
	prevRun := runMCPStdioServer
	runMCPStdioServer = func(ctx context.Context, _ *mcp.Server) error {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		return cancelCtx.Err()
	}
	defer func() {
		runMCPStdioServer = prevRun
	}()

	configPath := filepath.Join(t.TempDir(), "xsql.yaml")
	if err := os.WriteFile(configPath, []byte("profiles: {}\nssh_proxies: {}\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	GlobalConfig.ConfigStr = configPath
	err := runMCPServer(&mcpServerOptions{})
	if err != nil {
		t.Fatalf("expected nil error for canceled stdio server, got %v", err)
	}
}

func TestRunMCPServer_StdioPropagatesNonCanceledError(t *testing.T) {
	prevRun := runMCPStdioServer
	wantErr := context.DeadlineExceeded
	runMCPStdioServer = func(ctx context.Context, _ *mcp.Server) error {
		return wantErr
	}
	defer func() {
		runMCPStdioServer = prevRun
	}()

	configPath := filepath.Join(t.TempDir(), "xsql.yaml")
	if err := os.WriteFile(configPath, []byte("profiles: {}\nssh_proxies: {}\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	GlobalConfig.ConfigStr = configPath
	err := runMCPServer(&mcpServerOptions{})
	if err != wantErr {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestResolveMCPServerOptions_Defaults(t *testing.T) {
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
	}
	resolved, xe := resolveMCPServerOptions(nil, cfg)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.transport != "stdio" {
		t.Fatalf("expected stdio transport, got %s", resolved.transport)
	}
	if resolved.httpAddr != "127.0.0.1:8787" {
		t.Fatalf("expected default http addr, got %s", resolved.httpAddr)
	}
}

func TestResolveMCPServerOptions_StreamableHTTPEnv(t *testing.T) {
	t.Setenv("XSQL_MCP_TRANSPORT", "streamable_http")
	t.Setenv("XSQL_MCP_HTTP_AUTH_TOKEN", "env-token")
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
	}
	resolved, xe := resolveMCPServerOptions(&mcpServerOptions{}, cfg)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.transport != "streamable_http" {
		t.Fatalf("expected streamable_http transport, got %s", resolved.transport)
	}
	if resolved.httpAuthToken != "env-token" {
		t.Fatalf("expected env token, got %s", resolved.httpAuthToken)
	}
}

func TestResolveMCPServerOptions_StreamableHTTPConfigToken(t *testing.T) {
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
		MCP: config.MCPConfig{
			Transport: "streamable_http",
			HTTP: config.MCPHTTPConfig{
				Addr:                "127.0.0.1:9999",
				AuthToken:           "config-token",
				AllowPlaintextToken: true,
			},
		},
	}
	resolved, xe := resolveMCPServerOptions(&mcpServerOptions{}, cfg)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.httpAddr != "127.0.0.1:9999" {
		t.Fatalf("expected configured addr, got %s", resolved.httpAddr)
	}
	if resolved.httpAuthToken != "config-token" {
		t.Fatalf("expected config token, got %s", resolved.httpAuthToken)
	}
}

func TestResolveMCPServerOptions_InvalidTransport(t *testing.T) {
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
		MCP: config.MCPConfig{
			Transport: "bad",
		},
	}
	_, xe := resolveMCPServerOptions(&mcpServerOptions{}, cfg)
	if xe == nil {
		t.Fatal("expected error for invalid transport")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestResolveMCPServerOptions_StreamableHTTPMissingToken(t *testing.T) {
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
		MCP: config.MCPConfig{
			Transport: "streamable_http",
		},
	}
	_, xe := resolveMCPServerOptions(&mcpServerOptions{}, cfg)
	if xe == nil {
		t.Fatal("expected error for missing auth token")
	}
}

func TestResolveMCPServerOptions_EnvMissingToken(t *testing.T) {
	t.Setenv("XSQL_MCP_TRANSPORT", "streamable_http")
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
	}
	_, xe := resolveMCPServerOptions(&mcpServerOptions{}, cfg)
	if xe == nil {
		t.Fatal("expected error for missing auth token")
	}
}

func TestResolveMCPServerOptions_CLIOverridesEnvConfig(t *testing.T) {
	t.Setenv("XSQL_MCP_TRANSPORT", "streamable_http")
	t.Setenv("XSQL_MCP_HTTP_AUTH_TOKEN", "env-token")
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
		MCP: config.MCPConfig{
			Transport: "streamable_http",
			HTTP: config.MCPHTTPConfig{
				Addr:                "127.0.0.1:7000",
				AuthToken:           "config-token",
				AllowPlaintextToken: true,
			},
		},
	}
	opts := &mcpServerOptions{
		transport:        "stdio",
		transportSet:     true,
		httpAddr:         "127.0.0.1:6000",
		httpAddrSet:      true,
		httpAuthToken:    "cli-token",
		httpAuthTokenSet: true,
	}
	resolved, xe := resolveMCPServerOptions(opts, cfg)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.transport != "stdio" {
		t.Fatalf("expected stdio transport, got %s", resolved.transport)
	}
	if resolved.httpAddr != "127.0.0.1:6000" {
		t.Fatalf("expected CLI addr, got %s", resolved.httpAddr)
	}
	if resolved.httpAuthToken != "cli-token" {
		t.Fatalf("expected CLI token, got %s", resolved.httpAuthToken)
	}
}

func TestResolveMCPServerOptions_ConfigTokenPlaintextNotAllowed(t *testing.T) {
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
		MCP: config.MCPConfig{
			Transport: "streamable_http",
			HTTP: config.MCPHTTPConfig{
				AuthToken:           "config-token",
				AllowPlaintextToken: false,
			},
		},
	}
	_, xe := resolveMCPServerOptions(&mcpServerOptions{}, cfg)
	if xe == nil {
		t.Fatal("expected error for plaintext token without allow")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestMCPServerCommand_ConfigMissing(t *testing.T) {
	GlobalConfig.ConfigStr = filepath.Join(t.TempDir(), "missing.yaml")

	cmd := newMCPServerCommand()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestVersionCommand_Output(t *testing.T) {
	a := app.New("1.0.0", "abc", "2024-01-01")
	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	GlobalConfig.FormatStr = "json"

	cmd := NewVersionCommand(&a, &w)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("expected json output, got %s", out.String())
	}
}

func TestProfileShowCommand_ProfileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "xsql.yaml")
	configContent := `
profiles:
  dev:
    db: mysql
    host: localhost
    port: 3306
    user: root
    database: testdb
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	GlobalConfig.ConfigStr = configPath
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := newProfileShowCommand(&w)
	cmd.SetArgs([]string{"missing"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestResolveProxyPort(t *testing.T) {
	t.Run("nil cmd returns config port", func(t *testing.T) {
		port, fromConfig := resolveProxyPort(nil, &ProxyFlags{LocalPort: 5555}, 13306)
		if port != 13306 {
			t.Errorf("expected 13306, got %d", port)
		}
		if !fromConfig {
			t.Error("expected fromConfig=true")
		}
	})

	t.Run("nil cmd with zero config returns auto", func(t *testing.T) {
		port, fromConfig := resolveProxyPort(nil, &ProxyFlags{}, 0)
		if port != 0 {
			t.Errorf("expected 0, got %d", port)
		}
		if fromConfig {
			t.Error("expected fromConfig=false")
		}
	})

	t.Run("cli flag takes priority", func(t *testing.T) {
		cmd := NewProxyCommand(nil)
		// Simulate setting the flag
		_ = cmd.Flags().Set("local-port", "9999")
		port, fromConfig := resolveProxyPort(cmd, &ProxyFlags{LocalPort: 9999}, 13306)
		if port != 9999 {
			t.Errorf("expected 9999, got %d", port)
		}
		if fromConfig {
			t.Error("expected fromConfig=false")
		}
	})

	t.Run("config port when cli not set", func(t *testing.T) {
		cmd := NewProxyCommand(nil)
		// Don't set the flag - use config port
		port, fromConfig := resolveProxyPort(cmd, &ProxyFlags{}, 13306)
		if port != 13306 {
			t.Errorf("expected 13306, got %d", port)
		}
		if !fromConfig {
			t.Error("expected fromConfig=true")
		}
	})
}

func TestConfigInitCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")

	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := newConfigInitCommand(&w)
	cmd.SetArgs([]string{"--path", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config init failed: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("expected json output, got %s", out.String())
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file should exist: %v", err)
	}
}

func TestConfigInitCommand_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := newConfigInitCommand(&w)
	cmd.SetArgs([]string{"--path", path})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when file exists")
	}
}

func TestConfigSetCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	GlobalConfig.ConfigStr = path
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := newConfigSetCommand(&w)
	cmd.SetArgs([]string{"profile.dev.host", "localhost"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("expected json output, got %s", out.String())
	}

	// Verify the config was updated
	data, _ := os.ReadFile(path)
	if !bytes.Contains(data, []byte("localhost")) {
		t.Error("config should contain 'localhost'")
	}
}

func TestConfigSetCommand_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	GlobalConfig.ConfigStr = path
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := newConfigSetCommand(&w)
	cmd.SetArgs([]string{"badkey", "value"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestConfigSetCommand_NoConfig(t *testing.T) {
	GlobalConfig.ConfigStr = ""
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := newConfigSetCommand(&w)
	cmd.SetArgs([]string{"profile.dev.host", "localhost"})

	// Set HOME and work dir to temp dirs with no config files
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpWorkDir := t.TempDir()
	_ = os.Chdir(tmpWorkDir)
	defer func() { _ = os.Chdir(origDir) }()

	err := cmd.Execute()
	// FindConfigPath returns default home path, SetConfigValue creates the file.
	// This should either succeed (creating new file) or fail.
	// Since no config exists yet, it should succeed by creating a new one.
	if err != nil {
		// If it fails, that's okay too - we just want to verify it doesn't panic
		t.Logf("error (acceptable): %v", err)
	}
}

func TestRunProxy_WithConfigLocalPort(t *testing.T) {
	// Test that config local_port is used when --local-port is not set
	GlobalConfig.ProfileStr = "dev"
	GlobalConfig.FormatStr = "json"
	GlobalConfig.Resolved.Profile = config.Profile{
		DB:        "mysql",
		Host:      "db.example.com",
		Port:      3306,
		LocalPort: 13306,
		SSHConfig: &config.SSHProxy{
			Host: "bastion.example.com",
			Port: 22,
			User: "user",
		},
	}

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	// This will fail at SSH connection, but we can verify the port resolution
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error (SSH not available)")
	}
	// The error should be about SSH, not port
	if xe, ok := errors.As(err); ok && xe.Code == errors.CodePortInUse {
		t.Error("should not get port-in-use error")
	}
}

func TestValueIfSet(t *testing.T) {
	if got := valueIfSet(false, "x"); got != "" {
		t.Fatalf("expected empty when not set, got %q", got)
	}
	if got := valueIfSet(true, "x"); got != "x" {
		t.Fatalf("expected value when set, got %q", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "a", "b"); got != "a" {
		t.Fatalf("expected first non-empty value, got %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("expected empty when all empty, got %q", got)
	}
}

func configProfile(dbType string) config.Profile {
	return config.Profile{DB: dbType}
}

func TestHandlePortConflict_NonTTY(t *testing.T) {
	_, err := handlePortConflict(3306, "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for non-TTY port conflict")
	}
	if err.Code != errors.CodePortInUse {
		t.Fatalf("expected CodePortInUse, got %s", err.Code)
	}
}

func TestModeForWebCommand(t *testing.T) {
	if got := modeForWebCommand(true); got != "web" {
		t.Fatalf("expected 'web', got %q", got)
	}
	if got := modeForWebCommand(false); got != "serve" {
		t.Fatalf("expected 'serve', got %q", got)
	}
}

func TestResolveWebOptions_InvalidAddr(t *testing.T) {
	_, xe := resolveWebOptions(&webCommandOptions{
		addr:    "not-a-valid-addr",
		addrSet: true,
	}, config.File{})
	if xe == nil {
		t.Fatal("expected error for invalid addr")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestResolveWebOptions_EnvVars(t *testing.T) {
	t.Setenv("XSQL_WEB_HTTP_AUTH_TOKEN", "env-token")
	resolved, xe := resolveWebOptions(&webCommandOptions{
		addr:    "0.0.0.0:9999",
		addrSet: true,
	}, config.File{})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.authToken != "env-token" {
		t.Fatalf("expected env-token, got %s", resolved.authToken)
	}
	if !resolved.authRequired {
		t.Fatal("expected authRequired=true")
	}
}

func TestResolveMCPServerOptions_HttpAddrEnv(t *testing.T) {
	t.Setenv("XSQL_MCP_TRANSPORT", "streamable_http")
	t.Setenv("XSQL_MCP_HTTP_AUTH_TOKEN", "token")
	t.Setenv("XSQL_MCP_HTTP_ADDR", "127.0.0.1:5555")
	cfg := config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
	}
	resolved, xe := resolveMCPServerOptions(&mcpServerOptions{}, cfg)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.httpAddr != "127.0.0.1:5555" {
		t.Fatalf("expected 127.0.0.1:5555, got %s", resolved.httpAddr)
	}
}

func TestRunMCPServer_InvalidConfigPath(t *testing.T) {
	GlobalConfig.ConfigStr = "/nonexistent/path/config.yaml"
	err := runMCPServer(&mcpServerOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestRunMCPServer_StreamableHTTPStarts(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "xsql.yaml")
	if err := os.WriteFile(configPath, []byte("profiles: {}\nmcp:\n  transport: streamable_http\n  http:\n    addr: 127.0.0.1:0\n    auth_token: test-token\n    allow_plaintext_token: true\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	GlobalConfig.ConfigStr = configPath

	// This will start the HTTP server and then we need to stop it
	// We'll use a goroutine to run it and cancel after a short time
	done := make(chan error, 1)
	go func() {
		done <- runMCPServer(&mcpServerOptions{})
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// The server should be running, send SIGINT to stop it
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}

func TestNewServeCommand(t *testing.T) {
	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := NewServeCommand(&w)
	if cmd.Use != "serve" {
		t.Fatalf("expected 'serve', got %s", cmd.Use)
	}
}

func TestNewWebCommand(t *testing.T) {
	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := NewWebCommand(&w)
	if cmd.Use != "web" {
		t.Fatalf("expected 'web', got %s", cmd.Use)
	}
}

func TestNewMCPCommand(t *testing.T) {
	cmd := NewMCPCommand()
	if cmd.Use != "mcp" {
		t.Fatalf("expected 'mcp', got %s", cmd.Use)
	}
}

func TestNewProfileCommand(t *testing.T) {
	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := NewProfileCommand(&w)
	if cmd.Use != "profile" {
		t.Fatalf("expected 'profile', got %s", cmd.Use)
	}
}

func TestNewSchemaCommand(t *testing.T) {
	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := NewSchemaCommand(&w)
	if cmd.Use != "schema" {
		t.Fatalf("expected 'schema', got %s", cmd.Use)
	}
}

func TestResolveWebOptions_NilOpts(t *testing.T) {
	resolved, xe := resolveWebOptions(nil, config.File{})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.addr != "127.0.0.1:8788" {
		t.Fatalf("expected default addr, got %s", resolved.addr)
	}
}

func TestResolveWebOptions_ConfigAddr(t *testing.T) {
	resolved, xe := resolveWebOptions(&webCommandOptions{}, config.File{
		Web: config.WebConfig{
			HTTP: config.WebHTTPConfig{
				Addr: "127.0.0.1:9999",
			},
		},
	})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if resolved.addr != "127.0.0.1:9999" {
		t.Fatalf("expected 127.0.0.1:9999, got %s", resolved.addr)
	}
}

func TestResolveWebOptions_NonLoopbackRequiresToken(t *testing.T) {
	_, xe := resolveWebOptions(&webCommandOptions{
		addr:    "10.0.0.1:8788",
		addrSet: true,
	}, config.File{})
	if xe == nil {
		t.Fatal("expected error for non-loopback without token")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestRunProxy_InvalidFormat(t *testing.T) {
	prev := GlobalConfig
	GlobalConfig = &Config{ProfileStr: "dev", FormatStr: "invalid"}
	t.Cleanup(func() { GlobalConfig = prev })

	GlobalConfig.Resolved.Profile = config.Profile{DB: "mysql", SSHConfig: &config.SSHProxy{Host: "h", Port: 22, User: "u"}}

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestRunProxy_PlaintextNotAllowed(t *testing.T) {
	prev := GlobalConfig
	GlobalConfig = &Config{ProfileStr: "dev", FormatStr: "json"}
	t.Cleanup(func() { GlobalConfig = prev })

	GlobalConfig.Resolved.Profile = config.Profile{
		DB:             "mysql",
		Password:       "plain",
		AllowPlaintext: false,
		SSHConfig:      &config.SSHProxy{Host: "h", Port: 22, User: "u"},
	}

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runProxy(nil, &ProxyFlags{}, &w)
	if err == nil {
		t.Fatal("expected error for plaintext not allowed")
	}
}

func TestResolveProxyPort_AllPaths(t *testing.T) {
	// Test with config port and no CLI flag
	cmd := NewProxyCommand(nil)
	port, fromConfig := resolveProxyPort(cmd, &ProxyFlags{}, 5555)
	if port != 5555 || !fromConfig {
		t.Errorf("expected port=5555, fromConfig=true, got port=%d, fromConfig=%v", port, fromConfig)
	}

	// Test with zero config port
	port, fromConfig = resolveProxyPort(cmd, &ProxyFlags{}, 0)
	if port != 0 || fromConfig {
		t.Errorf("expected port=0, fromConfig=false, got port=%d, fromConfig=%v", port, fromConfig)
	}
}

func TestNewQueryCommand(t *testing.T) {
	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := NewQueryCommand(&w)
	if cmd.Use != "query [SQL]" {
		t.Fatalf("expected 'query [SQL]', got %s", cmd.Use)
	}
}

func TestNewProxyCommand(t *testing.T) {
	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	cmd := NewProxyCommand(&w)
	if cmd.Use != "proxy [flags]" {
		t.Fatalf("expected 'proxy [flags]', got %s", cmd.Use)
	}
}

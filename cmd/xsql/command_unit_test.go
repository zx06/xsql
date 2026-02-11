package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
	err := runQuery(nil, []string{"select 1"}, &QueryFlags{}, &w)
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
	err := runQuery(nil, []string{"select 1"}, &QueryFlags{}, &w)
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
	err := runQuery(nil, []string{"select 1"}, &QueryFlags{}, &w)
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
	err := runSchemaDump(nil, nil, &SchemaFlags{}, &w)
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
	err := runSchemaDump(nil, nil, &SchemaFlags{}, &w)
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
	err := runQuery(nil, []string{"select 1"}, &QueryFlags{}, &w)
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
	err := runSchemaDump(nil, nil, &SchemaFlags{}, &w)
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
	err := runSchemaDump(nil, nil, &SchemaFlags{}, &w)
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

func TestSetupSSH_NoConfig(t *testing.T) {
	client, err := setupSSH(nil, configProfile(""), false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Fatal("expected nil client")
	}
}

func TestSetupSSH_PassphraseResolveError(t *testing.T) {
	profile := config.Profile{
		DB: "mysql",
		SSHConfig: &config.SSHProxy{
			Host:       "example.com",
			Port:       22,
			User:       "user",
			Passphrase: "keyring:missing/passphrase",
		},
	}

	_, err := setupSSH(context.Background(), profile, false, false)
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

func configProfile(dbType string) config.Profile {
	return config.Profile{DB: dbType}
}

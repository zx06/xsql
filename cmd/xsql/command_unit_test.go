package main

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	xdb "github.com/zx06/xsql/internal/db"
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

func TestRunQuery_Success(t *testing.T) {
	driverName := registerStubDriver(t, map[string]*stubRows{
		"select 1": {
			columns: []string{"value"},
			rows:    [][]driver.Value{{1}},
		},
	})

	GlobalConfig.Resolved.Profile = config.Profile{
		DB: driverName,
	}
	GlobalConfig.FormatStr = "json"

	var out bytes.Buffer
	w := output.New(&out, &bytes.Buffer{})
	err := runQuery(nil, []string{"select 1"}, &QueryFlags{UnsafeAllowWrite: true}, &w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("expected json output, got %s", out.String())
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

type stubDriver struct {
	responseRows map[string]*stubRows
}

type stubConnector struct {
	driver *stubDriver
}

func (c *stubConnector) Connect(context.Context) (driver.Conn, error) {
	return &stubConn{driver: c.driver}, nil
}

func (c *stubConnector) Driver() driver.Driver {
	return c.driver
}

func (d *stubDriver) Open(string) (driver.Conn, error) {
	return &stubConn{driver: d}, nil
}

type stubConn struct {
	driver *stubDriver
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare not supported")
}

func (c *stubConn) Close() error {
	return nil
}

func (c *stubConn) Begin() (driver.Tx, error) {
	return &stubTx{}, nil
}

func (c *stubConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if rows, ok := c.driver.responseRows[query]; ok {
		return rows, nil
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type stubTx struct{}

func (t *stubTx) Commit() error {
	return nil
}

func (t *stubTx) Rollback() error {
	return nil
}

type stubRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *stubRows) Columns() []string {
	return r.columns
}

func (r *stubRows) Close() error {
	return nil
}

func (r *stubRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

func registerStubDriver(t *testing.T, rows map[string]*stubRows) string {
	t.Helper()

	name := fmt.Sprintf("stub-%d", time.Now().UnixNano())
	driver := &stubDriver{responseRows: rows}
	db := sql.OpenDB(&stubConnector{driver: driver})
	t.Cleanup(func() {
		_ = db.Close()
	})

	xdb.Register(name, fakeDriver{db: db})
	return name
}

type fakeDriver struct {
	db *sql.DB
}

func (d fakeDriver) Open(ctx context.Context, opts xdb.ConnOptions) (*sql.DB, *errors.XError) {
	return d.db, nil
}

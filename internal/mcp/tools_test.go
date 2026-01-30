package mcp

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

func TestCreateServer(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"test": {
				DB:     "mysql",
				Host:   "localhost",
				Port:   3306,
				User:   "test",
			},
		},
	}

	server, err := CreateServer("test", cfg)
	if err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("server is nil")
	}
}

func TestNewToolHandler(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB:              "mysql",
				Description:     "Dev database",
				UnsafeAllowWrite: false,
			},
			"prod": {
				DB:              "pg",
				Description:     "Prod database",
				UnsafeAllowWrite: true,
			},
		},
	}

	handler := NewToolHandler(cfg)
	if handler == nil {
		t.Fatal("handler is nil")
	}

	if handler.config == nil {
		t.Fatal("handler.config is nil")
	}

	if len(handler.config.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(handler.config.Profiles))
	}
}

func TestGetProfile_Default(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"default": {
				DB:   "mysql",
				Host: "localhost",
			},
			"other": {
				DB:   "pg",
				Host: "localhost",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Get default profile (empty name)
	profile := handler.getProfile("")
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}

	// Should return one of the profiles (order is not guaranteed)
	if profile.DB != "mysql" && profile.DB != "pg" {
		t.Errorf("expected db=mysql or db=pg, got %s", profile.DB)
	}
}

func TestGetProfile_ByName(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB:   "mysql",
				Host: "dev.example.com",
			},
			"prod": {
				DB:   "pg",
				Host: "prod.example.com",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Get dev profile
	profile := handler.getProfile("dev")
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}

	if profile.DB != "mysql" {
		t.Errorf("expected db=mysql, got %s", profile.DB)
	}

	if profile.Host != "dev.example.com" {
		t.Errorf("expected host=dev.example.com, got %s", profile.Host)
	}

	// Get prod profile
	profile = handler.getProfile("prod")
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}

	if profile.DB != "pg" {
		t.Errorf("expected db=pg, got %s", profile.DB)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Get non-existent profile
	profile := handler.getProfile("nonexistent")
	if profile != nil {
		t.Error("expected nil profile")
	}
}

func TestGetProfile_EmptyProfiles(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{},
	}

	handler := NewToolHandler(cfg)

	// Get default profile with empty config
	profile := handler.getProfile("")
	if profile != nil {
		t.Error("expected nil profile when no profiles exist")
	}
}

func TestFormatError(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{},
	}

	handler := NewToolHandler(cfg)

	// Test formatError with nil error (should handle gracefully)
	err := handler.formatError(nil)

	// Should return non-empty JSON
	if len(err) == 0 {
		t.Errorf("expected non-empty error output")
	}

	// Should contain valid JSON structure
	if len(err) > 0 && err[0] != '{' {
		t.Errorf("expected JSON format, got: %s", err)
	}
}

func TestProfileList(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB:              "mysql",
				Description:     "Dev database",
				UnsafeAllowWrite: false,
			},
			"prod": {
				DB:              "pg",
				Description:     "Prod database",
				UnsafeAllowWrite: true,
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Create a mock request
	req := &mcp.CallToolRequest{}

	result, _, err := handler.ProfileList(nil, req, struct{}{})
	if err != nil {
		t.Fatalf("ProfileList failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Error("expected content in result")
	}
}

func TestProfileShow_ProfileNotFound(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Test with non-existent profile
	result, _, err := handler.ProfileShow(nil, &mcp.CallToolRequest{}, ProfileShowInput{Name: "nonexistent"})
	if err != nil {
		t.Fatalf("ProfileShow failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for non-existent profile")
	}
}

func TestQuery_MissingSQL(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Test with missing SQL parameter
	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{Profile: "dev"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing SQL")
	}
}

func TestQuery_MissingProfile(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Test with missing profile parameter
	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{SQL: "SELECT 1"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing profile")
	}
}

func TestQuery_ProfileNotFound(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{
		SQL:     "SELECT 1",
		Profile: "nonexistent",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for non-existent profile")
	}
}

func TestQuery_MissingDBType(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "",
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{
		SQL:     "SELECT 1",
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing db type")
	}
}

func TestQuery_UnsupportedDBType(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "sqlite",
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{
		SQL:     "SELECT 1",
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unsupported db type")
	}
}

func TestQuery_InvalidPasswordFormat(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB:       "mysql",
				Password: "keyring:invalid:format:too:many:parts",
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{
		SQL:     "SELECT 1",
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should fail due to invalid password format
	if !result.IsError {
		t.Error("expected error for invalid password format")
	}
}

func TestQuery_WithSSHConfig(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
				SSHConfig: &config.SSHProxy{
					Host: "localhost",
					Port: 22,
					User: "test",
				},
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{
		SQL:     "SELECT 1",
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should fail due to SSH connection error
	if !result.IsError {
		t.Error("expected error for SSH connection failure")
	}
}

func TestQuery_WithSSHInvalidPassphrase(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
				SSHConfig: &config.SSHProxy{
					Host:       "localhost",
					Port:       22,
					User:       "test",
					Passphrase: "keyring:invalid:format",
				},
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.Query(nil, &mcp.CallToolRequest{}, QueryInput{
		SQL:     "SELECT 1",
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should fail due to invalid passphrase format
	if !result.IsError {
		t.Error("expected error for invalid passphrase format")
	}
}

func TestProfileShow_WithSSHProxy(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB:       "mysql",
				Host:     "localhost",
				Port:     3306,
				User:     "test",
				Password: "secret",
				SSHProxy: "bastion",
			},
		},
		SSHProxies: map[string]config.SSHProxy{
			"bastion": {
				Host:         "bastion.example.com",
				Port:         22,
				User:         "bastion",
				IdentityFile: "~/.ssh/id_rsa",
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.ProfileShow(nil, &mcp.CallToolRequest{}, ProfileShowInput{Name: "dev"})
	if err != nil {
		t.Fatalf("ProfileShow failed: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Error("expected content in result")
	}
}

func TestProfileShow_WithDSN(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB:   "mysql",
				DSN:  "user:pass@tcp(localhost:3306)/db",
				Host: "localhost",
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.ProfileShow(nil, &mcp.CallToolRequest{}, ProfileShowInput{Name: "dev"})
	if err != nil {
		t.Fatalf("ProfileShow failed: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	// Verify DSN is redacted
	content := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(content, `"dsn": "***"`) {
		t.Error("expected DSN to be redacted")
	}
}

func TestProfileShow_WithPassword(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB:       "mysql",
				Password: "mysecret",
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.ProfileShow(nil, &mcp.CallToolRequest{}, ProfileShowInput{Name: "dev"})
	if err != nil {
		t.Fatalf("ProfileShow failed: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	// Verify password is redacted
	content := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(content, `"password": "***"`) {
		t.Error("expected password to be redacted")
	}

	// Verify actual password is not exposed
	if strings.Contains(content, "mysecret") {
		t.Error("password should not be exposed in output")
	}
}

func TestProfileList_WithReadWriteProfile(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"readonly": {
				DB:              "mysql",
				Description:     "Read-only database",
				UnsafeAllowWrite: false,
			},
			"readwrite": {
				DB:              "pg",
				Description:     "Read-write database",
				UnsafeAllowWrite: true,
			},
		},
	}

	handler := NewToolHandler(cfg)

	result, _, err := handler.ProfileList(nil, &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("ProfileList failed: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	// Verify mode is correctly set
	content := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(content, `"mode": "read-only"`) {
		t.Error("expected read-only mode")
	}
	if !strings.Contains(content, `"mode": "read-write"`) {
		t.Error("expected read-write mode")
	}
}

func TestFormatError_WithXError(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{},
	}

	handler := NewToolHandler(cfg)

	// Test with a specific XError
	err := errors.New(errors.CodeCfgInvalid, "test error", map[string]interface{}{"key": "value"})
	result := handler.formatError(err)

	// Should contain error code, message, and details
	if !strings.Contains(result, "CFG_INVALID") {
		t.Errorf("expected error code in output, got: %s", result)
	}
	if !strings.Contains(result, "test error") {
		t.Errorf("expected error message in output, got: %s", result)
	}
	if !strings.Contains(result, "key") || !strings.Contains(result, "value") {
		t.Errorf("expected error details in output, got: %s", result)
	}
}

func TestFormatError_WithGenericError(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{},
	}

	handler := NewToolHandler(cfg)

	// Test with a generic error (non-XError)
	genericErr := &customError{msg: "something went wrong"}
	result := handler.formatError(genericErr)

	// Should be wrapped in XError format
	if !strings.Contains(result, `"ok": false`) {
		t.Error("expected ok: false in error output")
	}
	if !strings.Contains(result, `"error":`) {
		t.Error("expected error object in output")
	}
	if !strings.Contains(result, "something went wrong") {
		t.Error("expected error message in output")
	}
}

// customError is a simple error type for testing non-XError errors
type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func TestRegisterTools(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {
				DB: "mysql",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Create a mock server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test",
		Version: "1.0.0",
	}, nil)

	// Register tools should not panic
	handler.RegisterTools(server)

	// Server should have tools registered (basic sanity check)
	// We can't easily inspect registered tools without accessing private fields
	// But at least we verify it doesn't panic
}

func TestQuery_AllFields(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"full": {
				DB:               "mysql",
				Host:             "localhost",
				Port:             3306,
				User:             "root",
				Password:         "password",
				Database:         "testdb",
				UnsafeAllowWrite: true,
				AllowPlaintext:   true,
				Description:      "Full profile",
				DSN:              "root:password@tcp(localhost:3306)/testdb",
			},
		},
	}

	handler := NewToolHandler(cfg)

	// Test with all fields populated
	// This test requires actual database connection or mocking
	// For now, we'll just verify that the profile is correctly retrieved
	profile := handler.getProfile("full")
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}

	// Verify all fields are set
	if profile.DB != "mysql" {
		t.Errorf("expected DB=mysql, got %s", profile.DB)
	}
	if profile.Host != "localhost" {
		t.Errorf("expected Host=localhost, got %s", profile.Host)
	}
	if profile.Port != 3306 {
		t.Errorf("expected Port=3306, got %d", profile.Port)
	}
	if profile.User != "root" {
		t.Errorf("expected User=root, got %s", profile.User)
	}
	if !profile.UnsafeAllowWrite {
		t.Error("expected UnsafeAllowWrite=true")
	}
	if !profile.AllowPlaintext {
		t.Error("expected AllowPlaintext=true")
	}
	if profile.Description != "Full profile" {
		t.Errorf("expected Description=Full profile, got %s", profile.Description)
	}
}
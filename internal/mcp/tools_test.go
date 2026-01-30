package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zx06/xsql/internal/config"
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
package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/stats"
)

func TestSchemaListTables_MissingProfile(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaListTables(context.Background(), &mcp.CallToolRequest{}, SchemaListTablesInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for missing profile")
	}
}

func TestSchemaDump_MissingProfile(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaDump(context.Background(), &mcp.CallToolRequest{}, SchemaDumpInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for missing profile")
	}
}

func TestSchemaDescribeTable_MissingTable(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {DB: "mysql"},
		},
	}
	handler := NewToolHandler(cfg, stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaDescribeTable(context.Background(), &mcp.CallToolRequest{}, SchemaDescribeTableInput{
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for missing table")
	}
}

func TestSchemaDescribeTable_ProfileNotFound(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaDescribeTable(context.Background(), &mcp.CallToolRequest{}, SchemaDescribeTableInput{
		Profile: "missing",
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for missing profile")
	}
}

func TestRegisterTools_IncludesSchemaTools(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{"dev": {DB: "mysql"}},
	}
	handler := NewToolHandler(cfg, stats.StatsConfig{}, ServerOptions{})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	handler.RegisterTools(server)
}

func TestSchemaListTablesHandler_InvalidJSON(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: []byte("{")},
	}
	result, err := handler.schemaListTablesHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "CFG_INVALID") {
		t.Fatalf("expected CFG_INVALID, got %s", text)
	}
}

func emptyMCPConfig() *config.File {
	return &config.File{Profiles: map[string]config.Profile{}}
}

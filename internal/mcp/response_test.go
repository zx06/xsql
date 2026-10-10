package mcp

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/stats"
)

func TestSuccessResult_Envelope(t *testing.T) {
	handler := NewToolHandler(&config.File{Profiles: map[string]config.Profile{}}, stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.successResult(map[string]any{"n": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"ok": true`) || !strings.Contains(text, `"schema_version": 1`) {
		t.Fatalf("unexpected payload: %s", text)
	}
}

func TestSuccessResult_MarshalFailure(t *testing.T) {
	handler := NewToolHandler(&config.File{Profiles: map[string]config.Profile{}}, stats.StatsConfig{}, ServerOptions{})
	ch := make(chan int)
	result, _, err := handler.successResult(ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected marshal failure to become error result")
	}
	if !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "INTERNAL") {
		t.Fatalf("expected INTERNAL, got %s", result.Content[0].(*mcp.TextContent).Text)
	}
}

func TestErrorFromErr_Nil(t *testing.T) {
	handler := NewToolHandler(&config.File{Profiles: map[string]config.Profile{}}, stats.StatsConfig{}, ServerOptions{})
	result := handler.errorFromErr(nil)
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}
}

func TestErrorResult_WrapsXError(t *testing.T) {
	handler := NewToolHandler(&config.File{Profiles: map[string]config.Profile{}}, stats.StatsConfig{}, ServerOptions{})
	result := handler.errorResult(errors.New(errors.CodeCfgInvalid, "bad config", nil))
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}
	if !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "CFG_INVALID") {
		t.Fatalf("unexpected: %s", result.Content[0].(*mcp.TextContent).Text)
	}
}

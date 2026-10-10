package app

import (
	"testing"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

func TestResolveQueryMaxRows_CLIOverride(t *testing.T) {
	limit, xe := ResolveQueryMaxRows(config.Profile{MaxRows: 100}, "", 50, true)
	if xe != nil || limit != 50 {
		t.Fatalf("got limit=%d xe=%v", limit, xe)
	}
}

func TestResolveQueryMaxRows_CLIUnlimited(t *testing.T) {
	limit, xe := ResolveQueryMaxRows(config.Profile{MaxRows: 100}, "", 0, true)
	if xe != nil || limit != 0 {
		t.Fatalf("got limit=%d xe=%v", limit, xe)
	}
}

func TestResolveQueryMaxRows_ProfileAndDefault(t *testing.T) {
	limit, xe := ResolveQueryMaxRows(config.Profile{MaxRows: 200}, "", 0, false)
	if xe != nil || limit != 200 {
		t.Fatalf("got limit=%d xe=%v", limit, xe)
	}
	limit, xe = ResolveQueryMaxRows(config.Profile{}, "", 0, false)
	if xe != nil || limit != DefaultQueryMaxRows {
		t.Fatalf("got limit=%d xe=%v", limit, xe)
	}
}

func TestResolveQueryMaxRows_Invalid(t *testing.T) {
	_, xe := ResolveQueryMaxRows(config.Profile{}, "", -1, true)
	if xe == nil || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected cfg invalid, got %v", xe)
	}
}

func TestResolveQueryMaxRows_Env(t *testing.T) {
	limit, xe := ResolveQueryMaxRows(config.Profile{MaxRows: 100}, "250", 0, false)
	if xe != nil || limit != 250 {
		t.Fatalf("got limit=%d xe=%v", limit, xe)
	}
	_, xe = ResolveQueryMaxRows(config.Profile{}, "bad", 0, false)
	if xe == nil {
		t.Fatal("expected invalid env")
	}
}

func TestResolveMCPMaxResponseBytes(t *testing.T) {
	if got := ResolveMCPMaxResponseBytes(config.MCPConfig{}); got != DefaultMCPMaxResponseBytes {
		t.Fatalf("default bytes: %d", got)
	}
	if got := ResolveMCPMaxResponseBytes(config.MCPConfig{MaxResponseBytes: 1024}); got != 1024 {
		t.Fatalf("custom bytes: %d", got)
	}
}

func TestResolveMCPQueryMaxRows(t *testing.T) {
	v, xe := ResolveMCPQueryMaxRows(config.Profile{}, config.MCPConfig{}, 0, false)
	if xe != nil || v != DefaultMCPQueryMaxRows {
		t.Fatalf("default mcp: %d %v", v, xe)
	}
	_, xe = ResolveMCPQueryMaxRows(config.Profile{}, config.MCPConfig{}, 0, true)
	if xe == nil {
		t.Fatal("expected error for mcp max_rows 0")
	}
	v, xe = ResolveMCPQueryMaxRows(config.Profile{MaxRows: 800}, config.MCPConfig{MaxRows: 300}, 0, false)
	if xe != nil || v != 800 {
		t.Fatalf("profile override: %d %v", v, xe)
	}
}

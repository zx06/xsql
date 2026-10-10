package mcp

import (
	"testing"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/stats"
)

// MCP database tools use internal/app, which applies the same SSH option rules as CLI/Web.
func TestMCPQueryPathMatchesCLISSHOptions(t *testing.T) {
	cfg := config.File{
		Profiles: map[string]config.Profile{
			"remote": {
				DB:       "mysql",
				SSHProxy: "bastion",
			},
		},
		SSHProxies: map[string]config.SSHProxy{
			"bastion": {
				Host:        "jump.example.com",
				Port:        22,
				User:        "jump",
				SkipHostKey: true,
			},
		},
	}

	handler := NewToolHandler(&cfg, stats.StatsConfig{}, ServerOptions{})
	profile, xe := handler.resolveProfile("remote")
	if xe != nil {
		t.Fatalf("resolveProfile: %v", xe)
	}

	// MCP query uses SkipHostKeyCheck=false at runtime; profile skip_host_key still applies.
	mcpSSH, xe := app.SSHOptionsForProfile(profile, false, false)
	if xe != nil {
		t.Fatalf("SSHOptionsForProfile: %v", xe)
	}

	cliSSH, xe := app.SSHOptionsForProfile(profile, false, false)
	if xe != nil {
		t.Fatalf("SSHOptionsForProfile cli: %v", xe)
	}

	if mcpSSH.SkipKnownHostsCheck != cliSSH.SkipKnownHostsCheck {
		t.Fatalf("skip host key mismatch: mcp=%v cli=%v", mcpSSH.SkipKnownHostsCheck, cliSSH.SkipKnownHostsCheck)
	}
	if mcpSSH.Host != cliSSH.Host || mcpSSH.User != cliSSH.User {
		t.Fatalf("ssh target mismatch: mcp=%+v cli=%+v", mcpSSH, cliSSH)
	}
	if !mcpSSH.SkipKnownHostsCheck {
		t.Error("expected skip_host_key from profile to enable SkipKnownHostsCheck")
	}
}

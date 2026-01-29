package app

import (
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/spec"
)

type App struct {
	Version string
}

func New(version string) App {
	return App{Version: version}
}

func (a App) BuildSpec() spec.Spec {
	globalFlags := []spec.FlagSpec{
		{Name: "config", Default: "", Description: "Config file path (YAML); default: ./xsql.yaml or $HOME/.config/xsql/xsql.yaml"},
		{Name: "profile", Env: "XSQL_PROFILE", Default: "", Description: "Profile name (config: profiles.<name>)"},
		{Name: "format", Shorthand: "f", Env: "XSQL_FORMAT", Default: "auto", Description: "Output format: json|yaml|table|csv|auto"},
	}
	return spec.Spec{
		SchemaVersion: output.SchemaVersion,
		Commands: []spec.CommandSpec{
			{
				Name:        "spec",
				Description: "Export tool spec for AI/agents",
				Flags:       globalFlags,
			},
			{
				Name:        "version",
				Description: "Print version information",
				Flags:       globalFlags,
			},
			{
				Name:        "query",
				Description: "Execute a read-only SQL query",
				Flags: append(globalFlags,
					spec.FlagSpec{Name: "read-only", Default: "true", Description: "Enforce read-only mode"},
					spec.FlagSpec{Name: "unsafe-allow-write", Default: "false", Description: "Bypass read-only check (dangerous)"},
					spec.FlagSpec{Name: "allow-plaintext", Default: "false", Description: "Allow plaintext secrets in config"},
					spec.FlagSpec{Name: "ssh-skip-known-hosts-check", Default: "false", Description: "Skip SSH known_hosts check (dangerous)"},
				),
			},
		},
		ErrorCodes: errors.AllCodes(),
	}
}

type VersionInfo struct {
	Version string `json:"version" yaml:"version"`
}

func (a App) VersionInfo() VersionInfo {
	return VersionInfo{Version: a.Version}
}

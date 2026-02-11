package app

import (
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/spec"
)

type App struct {
	Version string
	Commit  string
	Date    string
}

func New(version, commit, date string) App {
	return App{Version: version, Commit: commit, Date: date}
}

func (a App) BuildSpec() spec.Spec {
	globalFlags := []spec.FlagSpec{
		{Name: "config", Default: "", Description: "Config file path (YAML); default: ./xsql.yaml or $HOME/.config/xsql/xsql.yaml"},
		{Name: "profile", Shorthand: "p", Env: "XSQL_PROFILE", Default: "", Description: "Profile name (config: profiles.<name>)"},
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
					spec.FlagSpec{Name: "unsafe-allow-write", Default: "false", Description: "Bypass read-only check (dangerous)"},
					spec.FlagSpec{Name: "allow-plaintext", Default: "false", Description: "Allow plaintext secrets in config"},
					spec.FlagSpec{Name: "ssh-skip-known-hosts-check", Default: "false", Description: "Skip SSH known_hosts check (dangerous)"},
				),
			},
			{
				Name:        "profile list",
				Description: "List all configured profiles",
				Flags:       globalFlags,
			},
			{
				Name:        "profile show",
				Description: "Show profile details (passwords are masked)",
				Flags:       globalFlags,
			},
			{
				Name:        "schema dump",
				Description: "Dump database schema (tables, columns, indexes, foreign keys)",
				Flags: append(globalFlags,
					spec.FlagSpec{Name: "table", Default: "", Description: "Table name filter (supports * and ? wildcards)"},
					spec.FlagSpec{Name: "include-system", Default: "false", Description: "Include system tables"},
					spec.FlagSpec{Name: "allow-plaintext", Default: "false", Description: "Allow plaintext secrets in config"},
					spec.FlagSpec{Name: "ssh-skip-known-hosts-check", Default: "false", Description: "Skip SSH known_hosts check (dangerous)"},
				),
			},
			{
				Name:        "proxy",
				Description: "Start a port forwarding proxy (replaces ssh -L)",
				Flags: append(globalFlags,
					spec.FlagSpec{Name: "local-port", Default: "0", Description: "Local port to listen on (0 for auto-assign)"},
					spec.FlagSpec{Name: "local-host", Default: "127.0.0.1", Description: "Local host to bind to"},
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
	Commit  string `json:"commit,omitempty" yaml:"commit,omitempty"`
	Date    string `json:"date,omitempty" yaml:"date,omitempty"`
}

func (a App) VersionInfo() VersionInfo {
	return VersionInfo(a)
}

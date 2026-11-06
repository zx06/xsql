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
		{Name: "attr", Env: "XSQL_ATTR", Default: "", Description: "Attribute key=value pair (repeatable)"},
	}
	connFlags := []spec.FlagSpec{
		{Name: "allow-plaintext", Default: "false", Description: "Allow plaintext secrets in config"},
		{Name: "ssh-skip-known-hosts-check", Default: "false", Description: "Skip SSH known_hosts check (dangerous)"},
	}
	webFlags := append([]spec.FlagSpec{
		{Name: "addr", Default: "127.0.0.1:8788", Env: "XSQL_WEB_HTTP_ADDR", Description: "Web HTTP listen address"},
		{Name: "auth-token", Default: "", Env: "XSQL_WEB_HTTP_AUTH_TOKEN", Description: "Bearer auth token required for non-loopback addresses"},
	}, connFlags...)
	statsJSONFlag := spec.FlagSpec{Name: "json", Default: "false", Description: "Output as JSON"}

	cmd := func(name, desc string, extra ...spec.FlagSpec) spec.CommandSpec {
		flags := make([]spec.FlagSpec, 0, len(globalFlags)+len(extra))
		flags = append(flags, globalFlags...)
		flags = append(flags, extra...)
		return spec.CommandSpec{Name: name, Description: desc, Flags: flags}
	}

	return spec.Spec{
		SchemaVersion: output.SchemaVersion,
		Commands: []spec.CommandSpec{
			cmd("spec", "Export tool spec for AI/agents"),
			cmd("version", "Print version information"),
			cmd("query", "Execute a read-only SQL query",
				spec.FlagSpec{Name: "unsafe-allow-write", Default: "false", Description: "Allow writes when profile unsafe_allow_write is true"},
				connFlags[0],
				connFlags[1],
				spec.FlagSpec{Name: "max-rows", Env: "XSQL_MAX_ROWS", Default: "1000", Description: "Maximum rows to return (0 = unlimited)"},
				spec.FlagSpec{Name: "query-timeout", Default: "30", Description: "Query timeout in seconds"},
			),
			cmd("profile list", "List all configured profiles"),
			cmd("profile show", "Show profile details (passwords are masked)"),
			cmd("schema dump", "Dump database schema (tables, columns, indexes, foreign keys)",
				spec.FlagSpec{Name: "table", Default: "", Description: "Table name filter (supports * and ? wildcards)"},
				spec.FlagSpec{Name: "include-system", Default: "false", Description: "Include system tables"},
				connFlags[0],
				connFlags[1],
				spec.FlagSpec{Name: "schema-timeout", Default: "60", Description: "Schema dump timeout in seconds"},
			),
			cmd("config init", "Create a template configuration file",
				spec.FlagSpec{Name: "path", Default: "", Description: "Config file path (default: $HOME/.config/xsql/xsql.yaml)"},
			),
			cmd("config set", "Set a configuration value (e.g., profile.dev.host localhost)"),
			cmd("mcp server", "Start MCP server for AI assistant integration",
				spec.FlagSpec{Name: "transport", Env: "XSQL_MCP_TRANSPORT", Default: "stdio", Description: "MCP transport: stdio|streamable_http"},
				spec.FlagSpec{Name: "http-addr", Env: "XSQL_MCP_HTTP_ADDR", Default: "127.0.0.1:8787", Description: "Streamable HTTP listen address"},
				spec.FlagSpec{Name: "http-auth-token", Env: "XSQL_MCP_HTTP_AUTH_TOKEN", Default: "", Description: "Streamable HTTP auth token (required for streamable_http)"},
				spec.FlagSpec{Name: "unsafe-allow-write", Env: "XSQL_MCP_UNSAFE_ALLOW_WRITE", Default: "false", Description: "Allow writes when profile unsafe_allow_write is true"},
			),
			cmd("proxy", "Start a port forwarding proxy (replaces ssh -L)",
				spec.FlagSpec{Name: "local-port", Default: "0", Description: "Local port to listen on (0 for auto-assign)"},
				spec.FlagSpec{Name: "local-host", Default: "127.0.0.1", Description: "Local host to bind to"},
				connFlags[0],
				connFlags[1],
			),
			cmd("serve", "Start the local web management server", webFlags...),
			cmd("web", "Start the local web management server and open a browser", webFlags...),
			cmd("stats", "Show usage statistics", statsJSONFlag),
			cmd("stats log", "Show detailed stats log",
				statsJSONFlag,
				spec.FlagSpec{Name: "limit", Default: "100", Description: "Maximum number of records to show"},
			),
			cmd("stats reset", "Reset usage statistics", statsJSONFlag),
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

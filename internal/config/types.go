package config

// File represents the xsql.yaml configuration structure.
// Constraint: config priority is CLI > ENV > Config.
type File struct {
	SSHProxies map[string]SSHProxy `yaml:"ssh_proxies"`
	Profiles   map[string]Profile  `yaml:"profiles"`
	MCP        MCPConfig           `yaml:"mcp"`
	Web        WebConfig           `yaml:"web"`
}

// SSHProxy defines a reusable SSH proxy configuration.
type SSHProxy struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	IdentityFile   string `yaml:"identity_file"`
	Passphrase     string `yaml:"passphrase"` // supports keyring:xxx reference
	KnownHostsFile string `yaml:"known_hosts_file"`
	SkipHostKey    bool   `yaml:"skip_host_key"` // strongly discouraged
}

type Profile struct {
	Description string `yaml:"description"` // description to distinguish databases
	Format      string `yaml:"format"`

	// DB connection
	DB       string `yaml:"db"`  // mysql | pg
	DSN      string `yaml:"dsn"` // raw DSN (takes precedence)
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"` // supports keyring:xxx reference
	Database string `yaml:"database"`

	// Security options
	AllowPlaintext   bool `yaml:"allow_plaintext"`    // allow plaintext password
	UnsafeAllowWrite bool `yaml:"unsafe_allow_write"` // allow write operations (bypass read-only protection)

	// Timeout settings (seconds)
	QueryTimeout  int `yaml:"query_timeout"`  // query timeout, default 30s
	SchemaTimeout int `yaml:"schema_timeout"` // schema export timeout, default 60s

	// SSH proxy reference (refers to a name defined in ssh_proxies)
	SSHProxy string `yaml:"ssh_proxy"`

	// Local port for the proxy (used by xsql proxy command)
	LocalPort int `yaml:"local_port"`

	// Resolved SSH config (populated by Resolve, not read from YAML)
	SSHConfig *SSHProxy `yaml:"-"`
}

// MCPConfig defines the MCP server configuration.
type MCPConfig struct {
	Transport string        `yaml:"transport"` // stdio | streamable_http
	HTTP      MCPHTTPConfig `yaml:"http"`
}

// MCPHTTPConfig defines the MCP Streamable HTTP transport configuration.
type MCPHTTPConfig struct {
	Addr                string `yaml:"addr"`
	AuthToken           string `yaml:"auth_token"`            // supports keyring:xxx reference
	AllowPlaintextToken bool   `yaml:"allow_plaintext_token"` // allow plaintext token
}

// WebConfig defines the local web server configuration.
type WebConfig struct {
	HTTP WebHTTPConfig `yaml:"http"`
}

// WebHTTPConfig defines the web HTTP transport configuration.
type WebHTTPConfig struct {
	Addr                string `yaml:"addr"`
	AuthToken           string `yaml:"auth_token"`            // supports keyring:xxx reference
	AllowPlaintextToken bool   `yaml:"allow_plaintext_token"` // allow plaintext token
}

type Resolved struct {
	ConfigPath  string
	ProfileName string
	Format      string
	Profile     Profile // full profile for query use
}

type Options struct {
	// ConfigPath: if non-empty, only this file is read (error if not found).
	ConfigPath string

	// CLI
	CLIProfile    string
	CLIProfileSet bool
	CLIFormat     string
	CLIFormatSet  bool

	// ENV (injected by caller for testability)
	EnvProfile string
	EnvFormat  string

	// HomeDir is used for default path resolution (auto-detected if empty).
	HomeDir string

	// WorkDir is used for default paths (falls back to process cwd if empty).
	WorkDir string
}

type ProfileInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DB          string `json:"db"`
	Mode        string `json:"mode"` // "read-only" or "read-write"
}

func ProfileToInfo(name string, p Profile) ProfileInfo {
	mode := "read-only"
	if p.UnsafeAllowWrite {
		mode = "read-write"
	}
	return ProfileInfo{
		Name:        name,
		Description: p.Description,
		DB:          p.DB,
		Mode:        mode,
	}
}

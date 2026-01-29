package config

// File 表示 xsql.yaml 的配置结构。
// 约束：配置优先级为 CLI > ENV > Config。
type File struct {
	SSHProxies map[string]SSHProxy `yaml:"ssh_proxies"`
	Profiles   map[string]Profile  `yaml:"profiles"`
}

// SSHProxy 定义可复用的 SSH 代理配置。
type SSHProxy struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	IdentityFile   string `yaml:"identity_file"`
	Passphrase     string `yaml:"passphrase"` // 支持 keyring:xxx 引用
	KnownHostsFile string `yaml:"known_hosts_file"`
	SkipHostKey    bool   `yaml:"skip_host_key"` // 极不推荐
}

type Profile struct {
	Format string `yaml:"format"`

	// DB 连接
	DB       string `yaml:"db"`  // mysql | pg
	DSN      string `yaml:"dsn"` // 原生 DSN（优先）
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"` // 支持 keyring:xxx 引用
	Database string `yaml:"database"`

	// 安全选项
	AllowPlaintext   bool `yaml:"allow_plaintext"`    // 允许明文密码
	UnsafeAllowWrite bool `yaml:"unsafe_allow_write"` // 允许写操作（绕过只读保护）

	// SSH proxy 引用（引用 ssh_proxies 中定义的名称）
	SSHProxy string `yaml:"ssh_proxy"`

	// 解析后的 SSH 配置（由 Resolve 填充，不从 YAML 读取）
	SSHConfig *SSHProxy `yaml:"-"`
}

type Resolved struct {
	ConfigPath  string
	ProfileName string
	Format      string
	Profile     Profile // 完整 profile 供 query 使用
}

type Options struct {
	// ConfigPath: 若非空，则只读取该文件（不存在报错）。
	ConfigPath string

	// CLI
	CLIProfile    string
	CLIProfileSet bool
	CLIFormat     string
	CLIFormatSet  bool

	// ENV（由调用方注入，便于测试）
	EnvProfile string
	EnvFormat  string

	// HomeDir 用于默认路径计算（为空则自动探测）。
	HomeDir string

	// WorkDir 用于默认路径（为空则使用进程当前工作目录）。
	WorkDir string
}

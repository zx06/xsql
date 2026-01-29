package config

// File 表示 xsql.yaml 的配置结构。
// 约束：配置优先级为 CLI > ENV > Config。
type File struct {
	Profiles map[string]Profile `yaml:"profiles"`
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
	ReadOnly bool   `yaml:"read_only"`

	// SSH proxy（可选）
	SSHHost           string `yaml:"ssh_host"`
	SSHPort           int    `yaml:"ssh_port"`
	SSHUser           string `yaml:"ssh_user"`
	SSHIdentityFile   string `yaml:"ssh_identity_file"`
	SSHPassphrase     string `yaml:"ssh_passphrase"` // 支持 keyring:xxx 引用
	SSHKnownHostsFile string `yaml:"ssh_known_hosts_file"`
	SSHSkipHostKey    bool   `yaml:"ssh_skip_host_key"` // 极不推荐
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

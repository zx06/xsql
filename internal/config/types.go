package config

// File 表示 xsql.yaml 的配置结构（第一阶段仅定义 profiles）。
// 约束：配置优先级为 CLI > ENV > Config。
type File struct {
	Profiles map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Format string `yaml:"format"`
}

type Resolved struct {
	ConfigPath  string
	ProfileName string
	Format      string
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

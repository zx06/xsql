package ssh

// Options 包含 SSH 连接所需参数。
type Options struct {
	Host           string
	Port           int
	User           string
	IdentityFile   string // 私钥路径
	Passphrase     string // 私钥 passphrase（若有）
	KnownHostsFile string // 默认 ~/.ssh/known_hosts

	// SkipKnownHostsCheck 跳过 known_hosts 校验（极不推荐！）
	SkipKnownHostsCheck bool
}

func DefaultKnownHostsPath() string {
	return "~/.ssh/known_hosts"
}

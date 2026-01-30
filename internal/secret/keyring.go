package secret

// KeyringAPI 是对 OS keyring 的最小抽象，便于测试与跨平台。
// service 对应 keyring 的 service name，account 对应 user/account。
type KeyringAPI interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

// 默认 keyring 实现（使用 zalando/go-keyring）
// 本文件仅定义接口；实现见 keyring_*.go（按平台编译）。
func defaultKeyring() KeyringAPI {
	return &osKeyring{}
}

type osKeyring struct{}

// Get/Set/Delete 见 keyring_default.go。

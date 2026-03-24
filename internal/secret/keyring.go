// Package secret handles credential resolution including OS keyring integration.
package secret

// KeyringAPI is a minimal abstraction over the OS keyring for testability and cross-platform support.
// service corresponds to the keyring service name; account corresponds to the user/account.
type KeyringAPI interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

// defaultKeyring returns the default keyring implementation (using zalando/go-keyring).
// This file only defines the interface; implementations are in keyring_*.go (platform-specific builds).
func defaultKeyring() KeyringAPI {
	return &osKeyring{}
}

type osKeyring struct{}

// Get/Set/Delete are implemented in keyring_default.go.

//go:build windows

package secret

import (
	"strings"

	"github.com/zalando/go-keyring"
)

func (o *osKeyring) Get(service, account string) (string, error) {
	val, err := keyring.Get(service, account)
	if err != nil {
		return "", err
	}
	// Windows cmdkey inserts null bytes between characters (UTF-16 legacy issue).
	val = strings.ReplaceAll(val, "\x00", "")
	return val, nil
}

func (o *osKeyring) Set(service, account, value string) error {
	return keyring.Set(service, account, value)
}

func (o *osKeyring) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

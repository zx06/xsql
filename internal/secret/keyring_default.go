package secret

import "github.com/zalando/go-keyring"

const serviceName = "xsql"

func (o *osKeyring) Get(key string) (string, error) {
	return keyring.Get(serviceName, key)
}

func (o *osKeyring) Set(key, value string) error {
	return keyring.Set(serviceName, key, value)
}

func (o *osKeyring) Delete(key string) error {
	return keyring.Delete(serviceName, key)
}

package secret

import (
	"strings"

	"github.com/zx06/xsql/internal/errors"
)

const keyringPrefix = "keyring:"

// Options controls secret resolution behavior.
type Options struct {
	AllowPlaintext bool       // whether to allow plaintext secrets (default false)
	Keyring        KeyringAPI // injectable keyring implementation (nil uses default)
}

const defaultService = "xsql"

// parseKeyringRef parses a keyring reference.
// The entire reference is used as the account; service is fixed to "xsql".
// For example, "prod/db_password" → service="xsql", account="prod/db_password".
func parseKeyringRef(ref string) (service, account string, err *errors.XError) {
	if ref == "" {
		return "", "", errors.New(errors.CodeCfgInvalid,
			"invalid keyring reference: empty account",
			map[string]any{"ref": ref})
	}
	return defaultService, ref, nil
}

// Resolve resolves a secret value following the order defined in docs/config.md:
//  1. keyring:<service>/<account> → read from the keyring
//  2. otherwise, if plaintext and plaintext is allowed → return as-is
//  3. otherwise → return an error
//
// Note: TTY interactive input is not implemented at this layer (left to the cmd layer).
func Resolve(raw string, opts Options) (string, *errors.XError) {
	if strings.HasPrefix(raw, keyringPrefix) {
		ref := strings.TrimPrefix(raw, keyringPrefix)
		service, account, parseErr := parseKeyringRef(ref)
		if parseErr != nil {
			return "", parseErr
		}
		kr := opts.Keyring
		if kr == nil {
			kr = defaultKeyring()
		}
		val, err := kr.Get(service, account)
		if err != nil {
			return "", errors.Wrap(errors.CodeSecretNotFound, "failed to read secret from keyring",
				map[string]any{"service": service, "account": account}, err)
		}
		return val, nil
	}
	// Plaintext
	if opts.AllowPlaintext {
		return raw, nil
	}
	return "", errors.New(errors.CodeCfgInvalid, "plaintext secret not allowed; use keyring: reference or enable --allow-plaintext", nil)
}

// IsKeyringRef reports whether s is a keyring reference.
func IsKeyringRef(s string) bool {
	return strings.HasPrefix(s, keyringPrefix)
}

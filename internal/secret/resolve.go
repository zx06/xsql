package secret

import (
	"strings"

	"github.com/zx06/xsql/internal/errors"
)

const keyringPrefix = "keyring:"

// Options 控制 secret 解析行为。
type Options struct {
	AllowPlaintext bool       // 是否允许明文（默认 false）
	Keyring        KeyringAPI // 可注入的 keyring 实现（nil 则用默认）
}

const defaultService = "xsql"

// parseKeyringRef 解析 keyring 引用。
// 整个引用作为 account，service 固定为 "xsql"。
// 例如 "prod/db_password" → service="xsql", account="prod/db_password"
func parseKeyringRef(ref string) (service, account string, err *errors.XError) {
	if ref == "" {
		return "", "", errors.New(errors.CodeCfgInvalid,
			"invalid keyring reference: empty account",
			map[string]any{"ref": ref})
	}
	return defaultService, ref, nil
}

// Resolve 解析 secret 值，遵循 docs/config.md 的顺序：
//  1. keyring:<service>/<account> → 从 keyring 读取
//  2. 否则若为明文且允许明文 → 直接返回
//  3. 否则报错
//
// 注意：TTY 交互输入本阶段不实现（留给 cmd 层处理）。
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
	// 明文
	if opts.AllowPlaintext {
		return raw, nil
	}
	return "", errors.New(errors.CodeCfgInvalid, "plaintext secret not allowed; use keyring: reference or enable --allow-plaintext", nil)
}

// IsKeyringRef 判断值是否为 keyring 引用。
func IsKeyringRef(s string) bool {
	return strings.HasPrefix(s, keyringPrefix)
}

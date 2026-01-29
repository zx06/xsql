package errors

// Code 是稳定错误码（字符串），供 AI/agent 与程序判断。
// 只增不改、不复用旧含义。
type Code string

const (
	// Config / args
	CodeCfgNotFound    Code = "XSQL_CFG_NOT_FOUND"
	CodeCfgInvalid     Code = "XSQL_CFG_INVALID"
	CodeSecretNotFound Code = "XSQL_SECRET_NOT_FOUND"

	// SSH
	CodeSSHAuthFailed      Code = "XSQL_SSH_AUTH_FAILED"
	CodeSSHHostKeyMismatch Code = "XSQL_SSH_HOSTKEY_MISMATCH"
	CodeSSHDialFailed      Code = "XSQL_SSH_DIAL_FAILED"

	// DB
	CodeDBDriverUnsupported Code = "XSQL_DB_DRIVER_UNSUPPORTED"
	CodeDBConnectFailed     Code = "XSQL_DB_CONNECT_FAILED"
	CodeDBAuthFailed        Code = "XSQL_DB_AUTH_FAILED"
	CodeDBExecFailed        Code = "XSQL_DB_EXEC_FAILED"

	// Read-only policy
	CodeROBlocked Code = "XSQL_RO_BLOCKED"

	// Internal
	CodeInternal Code = "XSQL_INTERNAL"
)

func AllCodes() []Code {
	return []Code{
		CodeCfgNotFound,
		CodeCfgInvalid,
		CodeSecretNotFound,
		CodeSSHAuthFailed,
		CodeSSHHostKeyMismatch,
		CodeSSHDialFailed,
		CodeDBDriverUnsupported,
		CodeDBConnectFailed,
		CodeDBAuthFailed,
		CodeDBExecFailed,
		CodeROBlocked,
		CodeInternal,
	}
}

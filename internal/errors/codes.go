package errors

// Code is a stable error code (string) for AI/agent and programmatic consumption.
// Codes are append-only; existing meanings must not be changed or reused.
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

	// Port
	CodePortInUse Code = "XSQL_PORT_IN_USE"

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
		CodePortInUse,
		CodeInternal,
	}
}

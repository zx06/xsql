package errors

// ExitCode 是进程退出码（稳定契约）；详见 docs/error-contract.md。
type ExitCode int

const (
	ExitOK ExitCode = 0

	// 2: 参数/配置错误
	ExitConfig ExitCode = 2

	// 3: 连接错误（DB/SSH）
	ExitConnect ExitCode = 3

	// 4: 只读策略拦截写入
	ExitReadOnly ExitCode = 4

	// 5: DB 执行错误
	ExitDBExec ExitCode = 5

	// 10: 内部错误
	ExitInternal ExitCode = 10
)

func ExitCodeFor(code Code) ExitCode {
	switch code {
	case CodeCfgNotFound, CodeCfgInvalid, CodeSecretNotFound:
		return ExitConfig
	case CodeSSHAuthFailed, CodeSSHHostKeyMismatch, CodeSSHDialFailed,
		CodeDBDriverUnsupported, CodeDBConnectFailed, CodeDBAuthFailed:
		return ExitConnect
	case CodeROBlocked:
		return ExitReadOnly
	case CodeDBExecFailed:
		return ExitDBExec
	case CodeInternal:
		fallthrough
	default:
		return ExitInternal
	}
}

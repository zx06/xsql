package errors

// ExitCode represents process exit codes (stable contract); see docs/error-contract.md.
type ExitCode int

const (
	ExitOK ExitCode = 0

	// 2: argument/configuration error
	ExitConfig ExitCode = 2

	// 3: connection error (DB/SSH)
	ExitConnect ExitCode = 3

	// 4: read-only policy blocked a write
	ExitReadOnly ExitCode = 4

	// 5: DB execution error
	ExitDBExec ExitCode = 5

	// 10: internal error
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
	case CodePortInUse:
		return ExitInternal
	case CodeDBExecFailed:
		return ExitDBExec
	case CodeInternal:
		fallthrough
	default:
		return ExitInternal
	}
}

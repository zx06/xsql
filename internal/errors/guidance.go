package errors

import (
	"strings"
)

const (
	detailHint           = "hint"
	detailRetryable      = "retryable"
	detailCategory       = "category"
	detailDialect        = "dialect"
	detailNativeCode     = "native_code"
	detailNativeCodeKind = "native_code_kind"
)

func enrichDetails(xe *XError) {
	if xe == nil {
		return
	}
	additions := guidanceFor(xe)
	if len(additions) == 0 {
		return
	}
	xe.Details = mergeDetails(xe.Details, additions)
}

func mergeDetails(base, additions map[string]any) map[string]any {
	if len(additions) == 0 {
		if base == nil {
			return nil
		}
		return base
	}
	out := make(map[string]any, len(base)+len(additions))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range additions {
		if _, exists := out[k]; exists {
			continue
		}
		out[k] = v
	}
	return out
}

func guidanceFor(xe *XError) map[string]any {
	switch xe.Code {
	case CodeROBlocked:
		return readOnlyGuidance(xe.Details)
	case CodeDBExecFailed:
		return dbExecGuidance(xe)
	case CodeDBConnectFailed, CodeDBAuthFailed:
		return connectGuidance(xe)
	case CodeSSHDialFailed, CodeSSHAuthFailed, CodeSSHHostKeyMismatch:
		return sshGuidance(xe)
	case CodeCfgNotFound, CodeCfgInvalid, CodeSecretNotFound:
		return configGuidance(xe)
	default:
		return nil
	}
}

func readOnlyGuidance(details map[string]any) map[string]any {
	out := map[string]any{
		detailCategory:  "read_only_blocked",
		detailRetryable: false,
		detailHint:      "Writes require profile `unsafe_allow_write: true` AND `--unsafe-allow-write` on `xsql query` / `xsql ai` or MCP server startup.",
	}
	if reason, _ := details["reason"].(string); reason != "" {
		if strings.HasPrefix(reason, "forbidden_keyword:") {
			out[detailHint] = "Statement contains a write keyword blocked by read-only policy; use SELECT-only SQL, or enable profile `unsafe_allow_write` and `--unsafe-allow-write` if the user explicitly requested writes."
		}
	}
	return out
}

func dbExecGuidance(xe *XError) map[string]any {
	dialect := dialectFromDetails(xe.Details)
	if xe.cause != nil {
		if c, ok := classifyDriverError(dialect, xe.cause); ok {
			return classificationToDetails(dialect, c)
		}
		if c, ok := classifyNetworkError(xe.cause); ok {
			return classificationToDetails(dialect, c)
		}
	}
	return map[string]any{
		detailCategory:  string(CategoryOther),
		detailRetryable: false,
		detailHint:      "Query execution failed; verify SQL against schema (try `xsql schema dump`) and dialect-specific syntax.",
	}
}

func connectGuidance(xe *XError) map[string]any {
	dialect := dialectFromDetails(xe.Details)
	out := map[string]any{
		detailCategory:  string(CategoryConnection),
		detailRetryable: true,
		detailHint:      "Database connection failed; check profile host, port, database name, credentials, and ssh_proxy settings.",
	}
	if dialect != "" {
		out[detailDialect] = dialect
	}
	if xe.cause != nil {
		if c, ok := classifyDriverError(dialect, xe.cause); ok {
			return classificationToDetails(dialect, c)
		}
		if c, ok := classifyNetworkError(xe.cause); ok {
			return classificationToDetails(dialect, c)
		}
		msg := strings.ToLower(xe.cause.Error())
		if xe.Code == CodeDBAuthFailed || strings.Contains(msg, "access denied") || strings.Contains(msg, "password authentication failed") {
			out[detailCategory] = string(CategoryPermissionDenied)
			out[detailRetryable] = false
			out[detailHint] = "Authentication failed; verify username/password or keyring secret for this profile (avoid plaintext in logs)."
		}
	}
	return out
}

func sshGuidance(xe *XError) map[string]any {
	out := map[string]any{
		detailCategory:  string(CategoryConnection),
		detailRetryable: true,
	}
	switch xe.Code {
	case CodeSSHAuthFailed:
		out[detailHint] = "SSH authentication failed; verify ssh user, private key, and keyring-stored passphrase for the profile or ssh_proxy."
		out[detailRetryable] = false
	case CodeSSHHostKeyMismatch:
		out[detailHint] = "SSH host key mismatch; update known_hosts or fix ssh host. Avoid `--ssh-skip-known-hosts-check` unless explicitly accepted."
		out[detailRetryable] = false
	default:
		out[detailHint] = "SSH dial failed; verify ssh_proxy host, port, and network path to the bastion."
	}
	if xe.cause != nil {
		if c, ok := classifyNetworkError(xe.cause); ok {
			if c.Hint != "" {
				out[detailHint] = c.Hint
			}
			out[detailRetryable] = c.Retryable
		}
	}
	return out
}

func configGuidance(xe *XError) map[string]any {
	out := map[string]any{
		detailRetryable: false,
		detailCategory:  "config",
	}
	switch xe.Code {
	case CodeCfgNotFound:
		out[detailHint] = "Config file missing; run `xsql config init` or pass `--config` with a valid path."
	case CodeSecretNotFound:
		out[detailHint] = "Secret not found in OS keyring; set the secret or use `--allow-plaintext` only when explicitly permitted."
	default:
		out[detailHint] = "Invalid configuration; run `xsql profile show <name>` and fix the reported field."
	}
	return out
}

func classificationToDetails(dialect string, c dbClassification) map[string]any {
	out := map[string]any{
		detailCategory:  string(c.Category),
		detailRetryable: c.Retryable,
	}
	if c.Hint != "" {
		out[detailHint] = c.Hint
	}
	if dialect != "" {
		out[detailDialect] = dialect
	}
	if c.NativeCode != "" {
		out[detailNativeCode] = c.NativeCode
	}
	if c.NativeCodeKind != "" {
		out[detailNativeCodeKind] = c.NativeCodeKind
	}
	return out
}

func dialectFromDetails(details map[string]any) string {
	if details == nil {
		return ""
	}
	if d, ok := details["dialect"].(string); ok {
		return normalizeDialect(d)
	}
	if d, ok := details["db"].(string); ok {
		return normalizeDialect(d)
	}
	return ""
}

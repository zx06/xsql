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

const (
	categoryReadOnlyBlocked     = "read_only_blocked"
	categoryUnrecognizedStmt    = "unrecognized_statement"
	hintROWriteAuthorization    = "Writes require profile `unsafe_allow_write: true` AND `--unsafe-allow-write` on `xsql query` or MCP server startup."
	hintROForbiddenKeyword      = "Statement contains a write keyword blocked by read-only policy; use SELECT-only SQL, or enable profile `unsafe_allow_write` and `--unsafe-allow-write` if the user explicitly requested writes."
	hintROForbiddenStart        = "Statement starts with a non-read-only command; use SELECT-only SQL, or enable profile `unsafe_allow_write` and `--unsafe-allow-write` if the user explicitly requested writes."
	hintROUnrecognizedStatement = "Statement not recognized as read-only SQL; check spelling and syntax (must start with an allowed read-only keyword such as SELECT)."
)

func readOnlyGuidance(details map[string]any) map[string]any {
	reason, _ := details["reason"].(string)
	if isReadOnlyUnrecognizedReason(reason) {
		return map[string]any{
			detailCategory:  categoryUnrecognizedStmt,
			detailRetryable: false,
			detailHint:      hintROUnrecognizedStatement,
		}
	}
	out := map[string]any{
		detailCategory:  categoryReadOnlyBlocked,
		detailRetryable: false,
		detailHint:      hintROWriteAuthorization,
	}
	switch {
	case strings.HasPrefix(reason, "forbidden_keyword:"):
		out[detailHint] = hintROForbiddenKeyword
	case strings.HasPrefix(reason, "forbidden_start:"):
		out[detailHint] = hintROForbiddenStart
	case strings.HasPrefix(reason, "cte_write_operation"):
		out[detailHint] = hintROForbiddenKeyword
	}
	return out
}

func isReadOnlyUnrecognizedReason(reason string) bool {
	switch reason {
	case "no_keyword", "non_letter_start", "empty", "empty_after_comment", "invalid_outer_paren", "multiple_statements":
		return true
	}
	return strings.HasPrefix(reason, "tokenize_error:")
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
			out[detailCategory] = string(CategoryAuthFailed)
			out[detailRetryable] = false
			out[detailHint] = hintAuthFailed
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
	if reason, _ := xe.Details["reason"].(string); reason == "table_not_found" {
		return map[string]any{
			detailCategory:  string(CategoryUnknownTable),
			detailRetryable: false,
			detailHint:      "Table not found; call MCP tool `schema_list_tables` (or `xsql schema dump` / `schema describe`) to list valid tables, then retry with the correct name or `table_pattern`.",
		}
	}

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

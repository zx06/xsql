package errors

// Shared agent hint strings (single source for Sonar duplicate-literal rules).
const (
	hintUnknownTableMySQL    = "Table or database not found; use MCP `schema_list_tables` or `xsql schema dump` to list valid tables, then fix the SQL."
	hintUnknownTablePG       = "Table or schema not found; use MCP `schema_list_tables` or `xsql schema dump` to list tables, then fix the SQL."
	hintUnknownColumn        = "Column not found; run `xsql schema dump --table <name>` to inspect columns, then fix the SQL."
	hintSyntaxMySQL          = "Fix SQL syntax; compare with the profile dialect (mysql) and re-run."
	hintSyntaxPG             = "Fix SQL syntax; compare with the profile dialect (pg) and re-run."
	hintPermissionDenied     = "Insufficient database privileges; use a read-only SELECT or ask for a profile with required grants."
	hintAuthFailed           = "Authentication failed; verify profile username/password or keyring secret reference for this profile."
	hintReadOnlyWrite        = "Write blocked in read-only mode; set profile `unsafe_allow_write: true` AND pass `--unsafe-allow-write` on query/ai/mcp server."
	hintReadOnlyWritePG      = "Write blocked in read-only transaction; set profile `unsafe_allow_write: true` AND pass `--unsafe-allow-write` on query/ai/mcp server."
	hintDuplicateKey         = "Duplicate key conflict; adjust INSERT/UPDATE values or use ON DUPLICATE handling if writes are explicitly allowed."
	hintDuplicateKeyPG       = "Duplicate key conflict; adjust values or constraints if writes are explicitly allowed."
	hintMySQLConnection      = "Cannot reach MySQL; verify profile host/port, network, and ssh_proxy settings."
	hintPGConnection         = "Cannot reach PostgreSQL; verify profile host/port, sslmode, and ssh_proxy settings."
	hintQueryTimeoutMySQL    = "Query timed out or lock wait exceeded; narrow the query, add LIMIT, or retry later."
	hintQueryTimeoutPG       = "Query canceled or lock timeout; narrow the query, add LIMIT, or retry later."
	hintSerializationPG      = "Serialization or deadlock failure; retry the read query or simplify concurrent access."
	hintDBOther              = "Database returned an error; inspect native_code and message, adjust SQL or connection settings."
	hintQueryResultTruncated = "Result truncated by max_rows; add SQL LIMIT/OFFSET, use aggregation, or run MCP `schema_list_tables` or `xsql schema dump` for schema discovery."
	hintMCPResponseTruncated = "Result truncated to fit MCP max_response_bytes; narrow the query with LIMIT or filters."
)

// HintQueryResultTruncated is the agent hint when query rows are capped (success data.hint).
const HintQueryResultTruncated = hintQueryResultTruncated

// HintMCPResponseTruncated is appended when MCP byte guard trims rows.
const HintMCPResponseTruncated = hintMCPResponseTruncated

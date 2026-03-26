---
name: xsql
description: Use when inspecting MySQL or PostgreSQL with the xsql CLI, especially for safe read-only querying, schema discovery, profile inspection, machine-readable JSON output, stable error-code handling, SSH-backed connections, or MCP server setup in this repository.
---

# xsql

Use `xsql` as an AI-first database CLI. Prefer it when the task requires stable JSON output, explicit error codes, schema discovery, or profile-aware access to MySQL/PostgreSQL.

## Default Operating Rules

- Prefer `--format json` for agent work, even if TTY would default to table.
- Treat `stdout` as data and `stderr` as logs; never parse logs as result data.
- Check `ok`, `schema_version`, and `error.code` instead of relying on human-readable text.
- Assume read-only mode unless the user explicitly requests write behavior.
- Do not suggest or use `--unsafe-allow-write` unless the task truly requires writes and the user has made that intent explicit.
- Do not leak secrets, full DSNs, passwords, private keys, or passphrases in output, logs, or summaries.
- Treat `--ssh-skip-known-hosts-check` as a risky last resort and call out the security tradeoff if it is required.

## Working Sequence

Use this sequence unless the user already knows the exact profile and SQL to run:

1. Identify the profile with `xsql profile list --format json`.
2. Inspect the chosen profile with `xsql profile show <profile> --format json`.
3. Discover schema with `xsql schema dump -p <profile> -f json`.
4. Write a minimal read-only query against the confirmed tables and columns.
5. Run `xsql query "<SQL>" -p <profile> -f json`.
6. Validate `ok`, `schema_version`, returned columns/rows, or `error.code`.

Prefer `schema dump` over guessing table names or dialect details.

## Command Patterns

```bash
# List available profiles
xsql profile list --format json

# Inspect a profile before querying
xsql profile show <profile> --format json

# Discover schema before writing SQL
xsql schema dump --profile <profile> --format json

# Filter schema discovery when the table family is known
xsql schema dump --profile <profile> --table "user*" --format json

# Run a read-only query
xsql query "SELECT id, name FROM users LIMIT 10" --profile <profile> --format json

# Export tool metadata for agent integration
xsql spec --format json

# Start MCP server
xsql mcp server
```

## Querying Guidance

- Start with schema discovery, then query.
- Keep queries narrow: explicit columns, predicates, and `LIMIT`.
- Match SQL syntax to the target engine after confirming whether the profile is MySQL or PostgreSQL.
- Use aggregation or sampling before asking for large result sets.
- Expect read-only enforcement to block writes through both SQL analysis and read-only transactions.

## Output And Error Handling

For machine-readable formats, expect:

```json
{"ok":true,"schema_version":1,"data":{...}}
```

```json
{"ok":false,"schema_version":1,"error":{"code":"...","message":"...","details":{...}}}
```

Handle failures by `error.code` and exit status, not by fragile string matching. Important exit codes in this repo:

- `0`: success
- `2`: argument/config error
- `3`: DB or SSH connection error
- `4`: read-only policy blocked a write
- `5`: database execution error
- `10`: internal error

Table and CSV output are for humans; they do not include `ok` or `schema_version`.

## Config And Profile Rules

- Resolve precedence as `CLI > ENV > Config`.
- Use `XSQL_`-prefixed env vars when environment configuration is needed.
- Default config lookup is `./xsql.yaml`, then `~/.config/xsql/xsql.yaml`, unless `--config` is provided.
- Prefer keyring-backed secrets. Plaintext secrets require explicit allowance.
- If no profile is passed and a `default` profile exists, xsql may use it automatically.

## SSH Rules

- Prefer the built-in SSH driver-dial path; it is the default design for MySQL/PostgreSQL.
- Use the local port-forwarding proxy mode only when the workflow explicitly needs `xsql proxy` semantics or a driver fallback.
- Keep host-key verification enabled by default.
- For one-shot commands such as `query` and `schema dump`, expect fresh SSH/DB connections rather than long-lived reconnect behavior.

## If You Are Modifying xsql

- Keep CLI-layer logic in `cmd/xsql` thin.
- Keep core behavior in `internal/*`, not coupled to Cobra types.
- Preserve output contracts and stable error codes.
- Add or update tests for JSON output, exit codes, and read-only behavior.

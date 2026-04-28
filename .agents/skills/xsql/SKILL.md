---
name: xsql
description: >
  AI-first database CLI for MySQL and PostgreSQL via xsql. Use this skill whenever the user wants to
  analyze, inspect, or query a database — including "分析数据库", "看下表结构", "查下数据量", "数据库概览",
  "表大小排行", "数据库健康检查", "碎片化分析", "查下索引", "慢查询", "优化建议", "表关系",
  schema discovery, data profiling, slow query investigation, index analysis, fragmentation check,
  foreign key analysis, or any task involving SQL databases. Also use when the user mentions xsql,
  database profiles, SSH-tunneled DB access, or read-only database operations. Trigger even if the
  user just names a database or environment (e.g. "生产环境数据库", "dev库", "帮我看下这个库",
  "staging DB") without explicitly saying "xsql". Also trigger for xsql web UI or MCP server setup.
---

# xsql

`xsql` is an AI-first database CLI that provides stable JSON output, explicit error codes, schema discovery, and profile-aware access to MySQL/PostgreSQL over SSH.

## Default Operating Rules

- Use `--format json` (`-f json`) for all agent-driven work — it returns structured `ok`/`data`/`error` envelopes. YAML format (`-f yaml`) is also available with the same envelope structure.
- `stdout` is data; `stderr` is logs. Never parse stderr as result data.
- Validate responses by checking `ok`, `schema_version`, and `error.code` — not by string matching.
- Assume read-only mode. Only suggest `--unsafe-allow-write` when the user has explicitly requested writes.
- Never leak secrets, full DSNs, passwords, or private keys in output or summaries.
- `--ssh-skip-known-hosts-check` is a risky last resort — call out the security tradeoff if it's needed.

## Working Sequence

Follow this sequence unless the user already knows the exact profile and SQL.

### Step 1: Resolve the profile

```bash
xsql profile list --format json
```

Match the user's intent to a profile by checking each profile's `description` and `database` fields. If the user says "生产环境" or "prod", look for a profile whose description contains those keywords. If ambiguous, ask the user to choose.

Then confirm the profile details:

```bash
xsql profile show <profile> --format json
```

Note the `db` field (`mysql` or `pg`) — it determines which SQL dialect to use. Also note the `database` name for use in queries.

### Step 2: Discover schema (adapt to database size)

**Small databases (under ~20 tables):** Full dump is fine.

```bash
xsql schema dump -p <profile> -f json
```

**Medium/large databases (20+ tables):** Full dump may be too large for context. Use a two-pass approach:

Pass 1 — Get table list only (lightweight, via information_schema):

```bash
# MySQL
xsql query "SELECT TABLE_NAME, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH FROM information_schema.TABLES WHERE TABLE_SCHEMA = '<database>' ORDER BY TABLE_ROWS DESC" -p <profile> -f json

# PostgreSQL
xsql query "SELECT relname AS table_name, reltuples::bigint AS row_estimate, pg_total_relation_size(relid) AS total_bytes FROM pg_stat_user_tables ORDER BY reltuples DESC" -p <profile> -f json
```

Pass 2 — Get detailed schema only for the tables you actually need:

```bash
xsql schema dump -p <profile> -f json --table "user*"
```

Additional schema dump flags: `--include-system` to include system tables, `--schema-timeout <seconds>` to override the default 60s timeout.

### Step 3: Write and run targeted queries

Write minimal, narrow queries with explicit columns, predicates, and `LIMIT`. Match SQL syntax to the profile's engine (MySQL vs PostgreSQL).

```bash
xsql query "<SQL>" -p <profile> -f json
```

For long-running queries, set a timeout (default: 30s):

```bash
xsql query "<SQL>" -p <profile> -f json --query-timeout 60
```

### Step 4: Validate the response

Check the JSON envelope:
- `ok: true` → use `data.rows` and `data.columns`
- `ok: false` → inspect `error.code` and `error.message`

## Health Check Workflow

When the user asks for a database analysis or health check, run multiple analysis patterns together. Run independent queries in parallel for efficiency.

The SQL patterns for each engine are in the reference files — read only the one matching the profile's `db` field:
- **MySQL**: Read `references/mysql-patterns.md`
- **PostgreSQL**: Read `references/postgresql-patterns.md`

### Recommended health check scope

For a comprehensive health check, include these areas in order of impact:

1. **Database Overview** — table sizes and row counts, identify the biggest tables
2. **Fragmentation Analysis** — detect wasted space and performance degradation
3. **Missing Index Detection** — find tables that need better indexing
4. **Stale Table Detection** — identify potentially abandoned tables

For specific investigations, also consider:
- **Growth Trend Analysis** — when the user asks about data growth or capacity planning
- **Column Distribution Analysis** — when the user asks about data distribution or skew
- **Server Status** — for deep performance diagnostics

## Querying Guidance

- Schema discovery first, query second — never guess table or column names.
- Keep queries narrow: explicit columns, WHERE predicates, and LIMIT.
- For large tables, prefer aggregation (`COUNT`, `GROUP BY`) over `SELECT *`.
- Use `--query-timeout` for long-running queries (default: 30s).
- Use `--schema-timeout` for large schema dumps (default: 60s).
- Run `xsql spec --format json` to discover all available commands and flags.

## Output And Error Handling

JSON responses follow this envelope:

```json
{"ok": true, "schema_version": 1, "data": {"columns": [...], "rows": [...]}}
```

```json
{"ok": false, "schema_version": 1, "error": {"code": "...", "message": "...", "details": {...}}}
```

Handle failures by `error.code` and exit status:

| Exit Code | Meaning |
|-----------|---------|
| 0 | success |
| 2 | argument/config error |
| 3 | DB or SSH connection error |
| 4 | read-only policy blocked a write |
| 5 | database execution error |
| 10 | internal error |

Table and CSV formats are for humans — they lack the `ok`/`schema_version` envelope.

## Additional Commands

```bash
# MCP server for AI assistant integration
xsql mcp server                                        # stdio transport (default)
xsql mcp server --transport streamable_http            # HTTP transport
xsql mcp server --transport streamable_http \
  --http-addr 127.0.0.1:8787 --http-auth-token <token> # HTTP with auth

# Web UI
xsql web                  # start web server and open browser
xsql serve                # start web server (headless, no browser)

# Configuration
xsql config init                                   # create template config file
xsql config set profile.dev.host localhost          # set a config value by dot-notation key

# Tool metadata and version
xsql spec --format json   # export tool spec for AI/agents
xsql version              # print version info
```

## Config And Profile Rules

- Precedence: `CLI flags > ENV vars > Config file`. ENV vars use `XSQL_` prefix (e.g. `XSQL_PROFILE`, `XSQL_FORMAT`).
- Config lookup: `./xsql.yaml`, then `~/.config/xsql/xsql.yaml`.
- Prefer keyring-backed secrets. Plaintext secrets require `--allow-plaintext`.
- If no profile is specified and a `default` profile exists, xsql uses it automatically.

## SSH Rules

- The built-in SSH driver-dial path is the default and preferred method.
- Use `xsql proxy` only when the workflow explicitly needs a local port-forward.
- Keep host-key verification enabled by default.
- One-shot commands (`query`, `schema dump`) use fresh connections — don't expect long-lived sessions.

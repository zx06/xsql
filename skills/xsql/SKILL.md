---
name: xsql
description: Execute read-only SQL queries on MySQL/PostgreSQL databases via SSH proxy. Use when needing to query databases, check data, explore database schemas, list profiles, or run SELECT statements.
---

# xsql - AI-first Cross-Database CLI Tool

xsql is a command-line tool designed for AI agents to execute read-only SQL queries on MySQL and PostgreSQL databases, with optional SSH tunnel support.

## Installation

Install the xsql binary:

```bash
go install github.com/zx06/xsql/cmd/xsql@latest
```

## Quick Reference

```bash
# Query with profile
xsql query "SELECT * FROM users LIMIT 10" -p dev -f json

# List all profiles
xsql profile list -f json

# Show profile details
xsql profile show dev -f json

# Get tool spec
xsql spec --format json

# Check version
xsql version
```

## Commands

### `xsql mcp server`

Start MCP (Model Context Protocol) server for AI assistant integration.

The MCP server exposes xsql's database query capabilities as MCP tools, allowing AI assistants (like Claude Desktop) to query databases through the standard MCP protocol.

**Available MCP Tools:**
- `query` - Execute SQL queries (profile required)
- `profile_list` - List all configured profiles
- `profile_show` - Show profile details

**Usage with Claude Desktop:**
```json
{
  "mcpServers": {
    "xsql": {
      "command": "xsql",
      "args": ["mcp", "server"],
      "env": {
        "XSQL_CONFIG": "/path/to/xsql.yaml"
      }
    }
  }
}
```

For detailed MCP integration information, see `docs/ai.md`.

### `xsql query <SQL>`

Execute a SQL query. **Read-only by default** to prevent accidental data modification.

**Read-only Protection (dual-layer, enabled by default):**
1. SQL static analysis (client-side) - blocks INSERT/UPDATE/DELETE/DROP keywords
2. Database transaction read-only mode (server-side) - uses `BEGIN READ ONLY` transaction

**To enable write operations:** Use `--unsafe-allow-write` flag or set `unsafe_allow_write: true` in profile config.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--profile` | `-p` | - | Profile name from config |
| `--format` | `-f` | auto | Output format: json/yaml/table/csv/auto |
| `--unsafe-allow-write` | - | false | Allow write operations (bypasses read-only protection) |
| `--allow-plaintext` | - | false | Allow plaintext secrets (or set `allow_plaintext: true` in config) |
| `--ssh-skip-known-hosts-check` | - | false | Skip SSH known_hosts check (dangerous) |

**Examples:**
```bash
# Read-only query (default)
xsql query "SELECT * FROM users" -p dev -f json

# Write operation
xsql query "INSERT INTO logs (msg) VALUES ('test')" -p dev --unsafe-allow-write
```

### `xsql spec`

Export tool specification for AI/agent discovery.

### `xsql version`

Print version information.

### `xsql profile list`

List all configured profiles.

**Output:**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "config_path": "/home/user/.config/xsql/xsql.yaml",
    "profiles": ["dev", "prod"]
  }
}
```

### `xsql profile show <name>`

Show profile details (passwords are masked).

**Output:**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "name": "dev",
    "db": "mysql",
    "host": "localhost",
    "port": 3306,
    "user": "root",
    "password": "***",
    "database": "mydb"
  }
}
```

## Output Formats

| Format | Use Case | Metadata |
|--------|----------|----------|
| `json` | AI/program consumption | Includes ok/schema_version |
| `yaml` | Human readable | Includes ok/schema_version |
| `table` | Terminal display | Data only |
| `csv` | Data export | Data only |
| `auto` | Auto-detect | TTY→table, otherwise→json |

## Response Format

### Success Response (JSON)

```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "columns": ["id", "name"],
    "rows": [
      {"id": 1, "name": "Alice"},
      {"id": 2, "name": "Bob"}
    ]
  }
}
```

### Error Response (JSON)

```json
{
  "ok": false,
  "schema_version": 1,
  "error": {
    "code": "XSQL_DB_CONNECT_FAILED",
    "message": "Failed to connect to database",
    "details": {}
  }
}
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Config/argument error |
| 3 | Connection error |
| 4 | Read-only policy blocked write |
| 5 | DB execution error |
| 10 | Internal error |

## Error Codes

| Code | Description |
|------|-------------|
| `XSQL_CFG_NOT_FOUND` | Config file not found |
| `XSQL_CFG_INVALID` | Invalid config format |
| `XSQL_SECRET_NOT_FOUND` | Secret not found in keyring |
| `XSQL_SSH_AUTH_FAILED` | SSH authentication failed |
| `XSQL_SSH_DIAL_FAILED` | SSH connection failed |
| `XSQL_DB_CONNECT_FAILED` | Database connection failed |
| `XSQL_DB_AUTH_FAILED` | Database authentication failed |
| `XSQL_DB_EXEC_FAILED` | SQL execution failed |
| `XSQL_RO_BLOCKED` | Write blocked by read-only mode |

## Configuration

Config file locations (in priority order):
1. `--config <path>` flag
2. `./xsql.yaml` (current directory)
3. `~/.config/xsql/xsql.yaml`

### Example Configuration

```yaml
# Define reusable SSH proxies
ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: ubuntu
    identity_file: ~/.ssh/id_rsa

profiles:
  dev:
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: keyring:dev/password
    database: mydb
  
  # Plaintext password example (set allow_plaintext: true to enable)
  local:
    db: mysql
    host: localhost
    port: 3306
    user: root
    password: secret123
    database: testdb
    allow_plaintext: true
  
  # Multiple profiles can share the same SSH proxy
  prod-db1:
    db: postgres
    host: db1.internal
    port: 5432
    user: readonly
    password: keyring:prod/password
    database: production
    ssh_proxy: bastion  # Reference the SSH proxy by name

  prod-db2:
    db: mysql
    host: db2.internal
    port: 3306
    user: app
    password: keyring:prod/db2_password
    database: analytics
    ssh_proxy: bastion  # Reuse the same proxy
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `XSQL_PROFILE` | Default profile name |
| `XSQL_FORMAT` | Default output format |

Priority: CLI flags > Environment variables > Config file

## Best Practices

1. **Always use `--format json`** for programmatic consumption
2. **Check the `ok` field** in response to determine success/failure
3. **Use exit codes** to detect error categories
4. **Parse error.code** for specific error handling
5. **Use profiles** to avoid hardcoding connection details
6. **Prefer read-only queries** - write operations are blocked by default

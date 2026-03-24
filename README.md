# xsql

[![CI](https://github.com/zx06/xsql/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zx06/xsql/actions/workflows/ci.yml?query=branch%3Amain)
[![codecov](https://codecov.io/github/zx06/xsql/graph/badge.svg?token=LrcR0pifCj)](https://codecov.io/github/zx06/xsql)
[![Go Reference](https://pkg.go.dev/badge/github.com/zx06/xsql.svg)](https://pkg.go.dev/github.com/zx06/xsql)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zx06/xsql)](https://github.com/zx06/xsql/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/zx06/xsql)](https://goreportcard.com/report/github.com/zx06/xsql)
[![License](https://img.shields.io/github/license/zx06/xsql)](https://github.com/zx06/xsql/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/zx06/xsql)](https://github.com/zx06/xsql/releases/latest)
[![GitHub Downloads](https://img.shields.io/github/downloads/zx06/xsql/total)](https://github.com/zx06/xsql/releases)
[![npm](https://img.shields.io/npm/v/xsql-cli)](https://www.npmjs.com/package/xsql-cli)
[![npm downloads](https://img.shields.io/npm/dm/xsql-cli)](https://www.npmjs.com/package/xsql-cli)

**Let AI safely query your databases** 🤖🔒

[中文文档](README_zh.md)

xsql is a cross-database CLI tool designed for AI agents. Read-only by default, structured output, ready out of the box.

```bash
# AI can query your database like this
xsql query "SELECT * FROM users WHERE created_at > '2024-01-01'" -p prod -f json
```

## ✨ Why xsql?

| Feature | Description |
|---------|-------------|
| 🔒 **Safe by Default** | Dual-layer read-only protection prevents accidental writes by AI |
| 🤖 **AI-first** | JSON structured output designed for machine consumption |
| 🔑 **Secure Credentials** | OS Keyring integration — passwords never touch disk |
| 🌐 **SSH Tunneling** | One-line config to connect to internal databases |
| 📦 **Zero Dependencies** | Single binary, works out of the box |

## 🚀 Quick Start

### 1. Install

```bash
# macOS
brew install zx06/tap/xsql

# Windows
scoop bucket add zx06 https://github.com/zx06/scoop-bucket && scoop install xsql

# npm / npx
npm install -g xsql-cli

# Or download directly: https://github.com/zx06/xsql/releases
```

### 2. Configure

```bash
mkdir -p ~/.config/xsql
cat > ~/.config/xsql/xsql.yaml << 'EOF'
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    password: your_password
    database: mydb
    allow_plaintext: true  # Use keyring for production
EOF
```

### 3. Use

```bash
xsql query "SELECT 1" -p dev -f json
# {"ok":true,"schema_version":1,"data":{"columns":["1"],"rows":[{"1":1}]}}
```

---

## 🤖 Let AI Use xsql

### Option 1: Claude Code Plugin (Recommended)

```bash
# 1. Add marketplace
/plugin marketplace add zx06/xsql

# 2. Install plugin
/plugin install xsql@xsql
```

After installation, Claude automatically gains xsql skills and can query databases directly.

### Option 2: Copy Skill Prompt to Any AI

Send the following to your AI assistant (ChatGPT/Claude/Cursor, etc.):

<details>
<summary>📋 Click to expand AI Skill Prompt (copy & paste)</summary>

```
You can now use the xsql tool to query databases.

## Basic Usage
xsql query "<SQL>" -p <profile> -f json

## Available Commands
- xsql query "SQL" -p <profile> -f json  # Execute query
- xsql schema dump -p <profile> -f json  # Export database schema
- xsql profile list -f json               # List all profiles
- xsql profile show <name> -f json        # Show profile details

## Output Format
Success: {"ok":true,"schema_version":1,"data":{"columns":[...],"rows":[...]}}
Failure: {"ok":false,"schema_version":1,"error":{"code":"XSQL_...","message":"..."}}

## Important Rules
1. Read-only mode by default — cannot execute INSERT/UPDATE/DELETE
2. Always use -f json for structured output
3. Use profile list to see available database configurations
4. Check the ok field to determine execution success

## Exit Codes
0=success, 2=config error, 3=connection error, 4=read-only violation, 5=SQL execution error
```

</details>

### Option 3: MCP Server (Claude Desktop, etc.)

Add the xsql MCP server to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "xsql": {
      "command": "xsql",
      "args": ["mcp", "server", "--config", "/path/to/xsql.yaml"]
    }
  }
}
```

Once started, Claude can query databases directly via the MCP protocol.

### Option 4: AGENTS.md / Rules (Cursor/Windsurf)

Create `.cursor/rules` or edit `AGENTS.md` in your project root:

```markdown
## Database Queries

Use xsql to query databases:
- Query: `xsql query "SELECT ..." -p <profile> -f json`
- Export schema: `xsql schema dump -p <profile> -f json`
- List profiles: `xsql profile list -f json`

Note: Read-only mode by default. Write operations require the --unsafe-allow-write flag.
```

---

## 📖 Features

### Command Reference

| Command | Description |
|---------|-------------|
| `xsql query <SQL>` | Execute SQL queries (read-only by default) |
| `xsql schema dump` | Export database schema (tables, columns, indexes, foreign keys) |
| `xsql profile list` | List all profiles |
| `xsql profile show <name>` | Show profile details (passwords are masked) |
| `xsql mcp server` | Start MCP Server (AI assistant integration) |
| `xsql spec` | Export AI Tool Spec (supports `--format yaml`) |
| `xsql version` | Show version information |

### Output Formats

```bash
# JSON (for AI/programs)
xsql query "SELECT id, name FROM users" -p dev -f json
{"ok":true,"schema_version":1,"data":{"columns":["id","name"],"rows":[{"id":1,"name":"Alice"}]}}

# Table (for terminals)
xsql query "SELECT id, name FROM users" -p dev -f table
id  name
--  -----
1   Alice

(1 rows)
```

### Schema Discovery (AI auto-understands your database)

```bash
# Export database schema (for AI to understand table structures)
xsql schema dump -p dev -f json

# Filter specific tables
xsql schema dump -p dev --table "user*" -f json

# Example output
{
  "ok": true,
  "data": {
    "database": "mydb",
    "tables": [
      {
        "name": "users",
        "columns": [
          {"name": "id", "type": "bigint", "primary_key": true},
          {"name": "email", "type": "varchar(255)", "nullable": false}
        ]
      }
    ]
  }
}
```

### SSH Tunnel Connection

```yaml
ssh_proxies:
  bastion:
    host: jump.example.com
    user: admin
    identity_file: ~/.ssh/id_ed25519

profiles:
  prod:
    db: pg
    host: db.internal  # Internal network address
    port: 5432
    user: readonly
    password: "keyring:prod/password"
    database: mydb
    ssh_proxy: bastion  # Reference SSH proxy
```

### Port Forwarding Proxy (xsql proxy)

When you need traditional `ssh -L` behavior or want to expose a local port for GUI clients, use `xsql proxy`:

```bash
# Start port forwarding (auto-assign local port)
xsql proxy -p prod

# Specify local port
xsql proxy -p prod --local-port 13306
```

> This command requires the profile to have `ssh_proxy` configured. It listens on a local port and forwards traffic to the target database.

### Security Features

- **Dual-layer Read-only Protection**: SQL static analysis + database transaction-level read-only
- **Keyring Integration**: `password: "keyring:prod/password"`
- **Password Masking**: `profile show` never exposes passwords
- **SSH Security**: known_hosts verification enabled by default

---

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [CLI Specification](docs/cli-spec.md) | Detailed CLI interface reference |
| [Configuration Guide](docs/config.md) | Config file format and options |
| [SSH Proxy](docs/ssh-proxy.md) | SSH tunnel configuration |
| [Error Handling](docs/error-contract.md) | Error codes and exit codes |
| [AI Integration](docs/ai.md) | MCP Server and AI assistant integration |
| [RFC Documents](docs/rfcs/) | Design change records |
| [Development Guide](docs/dev.md) | Contributing and development notes |

---

## License

[MIT](LICENSE)

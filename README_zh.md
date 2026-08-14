# xsql

[![CI](https://github.com/zx06/xsql/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zx06/xsql/actions/workflows/ci.yml?query=branch%3Amain)
[![codecov](https://codecov.io/github/zx06/xsql/graph/badge.svg?token=LrcR0pifCj)](https://codecov.io/github/zx06/xsql)
[![Go Reference](https://pkg.go.dev/badge/github.com/zx06/xsql.svg)](https://pkg.go.dev/github.com/zx06/xsql)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zx06/xsql)](https://github.com/zx06/xsql/blob/main/go.mod)
[![GitHub Stars](https://img.shields.io/github/stars/zx06/xsql)](https://github.com/zx06/xsql/stargazers)
[![License](https://img.shields.io/github/license/zx06/xsql)](https://github.com/zx06/xsql/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/zx06/xsql)](https://github.com/zx06/xsql/releases/latest)
[![GitHub Downloads](https://img.shields.io/github/downloads/zx06/xsql/total)](https://github.com/zx06/xsql/releases)
[![npm](https://img.shields.io/npm/v/xsql-cli)](https://www.npmjs.com/package/xsql-cli)
[![npm downloads](https://img.shields.io/npm/dm/xsql-cli)](https://www.npmjs.com/package/xsql-cli)

**让 AI 安全地查询你的数据库** 🤖🔒

[English](README.md)

xsql 是专为 AI Agent 设计的跨数据库 CLI 工具。默认只读、结构化输出、开箱即用。

```bash
# AI 可以这样查询你的数据库
xsql query "SELECT * FROM users WHERE created_at > '2024-01-01'" -p prod -f json --attr source=codex-cli --attr agent=codex --attr env=prod --attr task=targeted-query
```

## ✨ 为什么选择 xsql？

| 特性 | 说明 |
|------|------|
| 🔒 **默认安全** | 双重只读保护，防止 AI 误操作 |
| 🤖 **AI-first** | JSON 结构化输出，便于 AI 解析 |
| 🔑 **密钥安全** | 集成 OS Keyring，密码不落盘 |
| 🌐 **SSH 隧道** | 一行配置连接内网数据库 |
| 📦 **零依赖** | 单二进制文件，开箱即用 |

## 🚀 30 秒上手

### 1. 安装

```bash
# macOS
brew install zx06/tap/xsql

# Windows
scoop bucket add zx06 https://github.com/zx06/scoop-bucket && scoop install xsql

# npm / npx
npm install -g xsql-cli

# 或直接下载: https://github.com/zx06/xsql/releases
```

### 2. 配置

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
    allow_plaintext: true  # 生产环境建议用 keyring
EOF
```

### 3. 使用

```bash
xsql query "SELECT 1" -p dev -f json
# {"ok":true,"schema_version":1,"data":{"columns":["1"],"rows":[{"1":1}]}}
```

---

## 🤖 让 AI 使用 xsql

### 方式一：Agent Skills Directory（推荐）

通过 [The Agent Skills Directory](https://www.skills.sh/) 安装：

```bash
npx skills add zx06/xsql
```

### 方式二：Claude Code Plugin

```bash
# 1. 添加 marketplace
/plugin marketplace add zx06/xsql

# 2. 安装插件
/plugin install xsql@xsql
```

安装后 Claude 自动获得 xsql 技能，可直接查询数据库。

### 方式三：复制 Skill 给任意 AI

将以下内容发送给你的 AI 助手（ChatGPT/Claude/Cursor 等）：

<details>
<summary>📋 点击展开 AI Skill Prompt（复制即用）</summary>

```
你现在可以使用 xsql 工具查询数据库。

## 基本用法
xsql query "<SQL>" -p <profile> -f json --attr source=codex-cli --attr agent=codex --attr env=<env> --attr task=<task>

## 可用命令
- xsql query "SQL" -p <profile> -f json --attr source=codex-cli --attr agent=codex --attr env=<env> --attr task=<task> # 执行查询
- xsql schema dump -p <profile> -f json --attr source=codex-cli --attr agent=codex --attr env=<env> --attr task=schema-discovery # 导出数据库结构
- xsql profile list -f json --attr source=codex-cli --attr agent=codex --attr task=profile-discovery # 列出所有 profile
- xsql profile show <name> -f json --attr source=codex-cli --attr agent=codex --attr task=profile-discovery # 查看 profile 详情

## 输出格式
成功: {"ok":true,"schema_version":1,"data":{"columns":[...],"rows":[...]}}
失败: {"ok":false,"schema_version":1,"error":{"code":"XSQL_...","message":"..."}}

## 重要规则
1. 默认只读；写入要求 profile 设置 `unsafe_allow_write: true`，且本次命令同时携带 `--unsafe-allow-write`
2. 始终使用 -f json 获取结构化输出
3. Codex 发起的每次 xsql 调用都添加 `--attr source=codex-cli`，并在已知时补充 `agent`、`env`、`team`、`task`
4. 先用 profile list 查看可用的数据库配置
5. 检查 ok 字段判断执行是否成功

## 退出码
0=成功, 2=配置错误, 3=连接错误, 4=只读拦截, 5=SQL执行错误
```

</details>

### 方式四：MCP Server（Claude Desktop 等）

在 Claude Desktop 配置中添加 xsql MCP server：

```json
{
  "mcpServers": {
    "xsql": {
      "command": "xsql",
      "args": ["mcp", "server", "--config", "/path/to/xsql.yaml", "--attr", "source=codex-cli", "--attr", "agent=codex"]
    }
  }
}
```

启动后，Claude 可以直接通过 MCP 协议查询数据库。

### 方式五：AGENTS.md / Rules（Cursor/Windsurf）

在项目根目录创建 `.cursor/rules` 或编辑 `AGENTS.md`：

```markdown
## 数据库查询

使用 xsql 工具查询数据库：
- 查询: `xsql query "SELECT ..." -p <profile> -f json --attr source=codex-cli --attr agent=codex --attr env=<env> --attr task=targeted-query`
- 导出结构: `xsql schema dump -p <profile> -f json --attr source=codex-cli --attr agent=codex --attr env=<env> --attr task=schema-discovery`
- 列出配置: `xsql profile list -f json --attr source=codex-cli --attr agent=codex --attr task=profile-discovery`

注意: 默认只读；写入要求 profile 设置 `unsafe_allow_write: true`，且本次命令同时携带 `--unsafe-allow-write`。
```

---

## 📖 功能详情

### 命令一览

| 命令 | 说明 |
|------|------|
| `xsql ai [PROMPT]` | 启动全屏交互式 AI 终端模式（TUI） |
| `xsql query <SQL>` | 执行 SQL 查询（默认只读） |
| `xsql schema dump` | 导出数据库结构（表、列、索引、外键） |
| `xsql profile list` | 列出所有 profile |
| `xsql profile show <name>` | 查看 profile 详情（密码脱敏） |
| `xsql mcp server` | 启动 MCP Server（AI 助手集成） |
| `xsql config init/set` | 创建或更新配置文件 |
| `xsql proxy` | 启动 SSH 本地端口转发代理 |
| `xsql serve` / `xsql web` | 启动本地 Web UI |
| `xsql stats` | 查看使用统计与审计属性 |
| `xsql spec` | 导出 AI Tool Spec（支持 `--format yaml`） |
| `xsql version` | 显示版本信息 |

### 输出格式

```bash
# JSON（AI/程序）
xsql query "SELECT id, name FROM users" -p dev -f json
{"ok":true,"schema_version":1,"data":{"columns":["id","name"],"rows":[{"id":1,"name":"Alice"}]}}

# Table（终端）
xsql query "SELECT id, name FROM users" -p dev -f table
id  name
--  -----
1   Alice

(1 rows)
```

### Schema 发现（AI 自动理解数据库）

```bash
# 导出数据库结构（供 AI 理解表结构）
xsql schema dump -p dev -f json

# 过滤特定表
xsql schema dump -p dev --table "user*" -f json

# 输出示例
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

### SSH 隧道连接

```yaml
ssh_proxies:
  bastion:
    host: jump.example.com
    user: admin
    identity_file: ~/.ssh/id_ed25519

profiles:
  prod:
    db: pg
    host: db.internal  # 内网地址
    port: 5432
    user: readonly
    password: "keyring:prod/password"
    database: mydb
    ssh_proxy: bastion  # 引用 SSH 代理
```

### 端口转发代理（xsql proxy）

当需要传统的 `ssh -L` 行为或希望暴露本地端口给 GUI 客户端时，可以使用 `xsql proxy`：

```bash
# 启动端口转发（自动分配本地端口）
xsql proxy -p prod

# 指定本地端口
xsql proxy -p prod --local-port 13306
```

> 该命令要求 profile 配置 `ssh_proxy`，并会在本地监听端口，将流量转发到目标数据库。

### 安全特性

- **双重只读保护**：SQL 静态分析 + 数据库事务级只读
- **双重写入授权**：CLI 写入同时要求 profile 授权与本次命令 flag
- **Keyring 集成**：`password: "keyring:prod/password"`
- **密码脱敏**：`profile show` 不泄露密码
- **SSH 安全**：默认验证 known_hosts

---

## 📚 文档

| 文档 | 说明 |
|------|------|
| [CLI 规范](docs/cli-spec.md) | 命令行接口详细说明 |
| [配置指南](docs/config.md) | 配置文件格式和选项 |
| [SSH 代理](docs/ssh-proxy.md) | SSH 隧道配置 |
| [错误处理](docs/error-contract.md) | 错误码和退出码 |
| [AI 集成](docs/ai.md) | MCP Server 和 AI 助手集成 |
| [RFC 文档](docs/rfcs/) | 设计变更记录 |
| [开发指南](docs/dev.md) | 贡献和开发说明 |

---

## License

[MIT](LICENSE)

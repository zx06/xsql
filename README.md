# xsql

[![CI](https://github.com/zx06/xsql/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zx06/xsql/actions/workflows/ci.yml?query=branch%3Amain)
[![codecov](https://codecov.io/github/zx06/xsql/graph/badge.svg?token=LrcR0pifCj)](https://codecov.io/github/zx06/xsql)

AI-first 的跨数据库 CLI 工具（Golang）。

## Claude Code Plugin

本仓库也是一个 [Claude Code Plugin](https://docs.anthropic.com/en/docs/claude-code/plugins)，可通过以下方式安装：

```bash
# 添加 marketplace
/plugin marketplace add zx06/xsql

# 安装插件
/plugin install xsql@xsql
```

安装后，Claude Code 将自动获得 xsql 工具的使用技能。

## 目标
- 为 AI agent 提供稳定、可机读、可组合的数据库操作接口（CLI/未来 server/MCP）。
- 支持 MySQL / PostgreSQL，并具备可扩展的 driver 架构。
- 支持 SSH proxy（driver 自定义 dial）。

## 功能
- `xsql query` - 执行 SQL 查询（支持 MySQL / PostgreSQL）
  - **默认只读**：双重保护（SQL 静态分析 + 数据库事务级只读）
  - **可启用写操作**：`--unsafe-allow-write` 或配置 `unsafe_allow_write: true`
- `xsql spec` - 导出 tool spec（供 AI/agent 自动发现）
- `xsql version` - 版本信息
- `xsql profile list` - 列出所有配置的 profiles
- `xsql profile show <name>` - 查看 profile 详情（密码脱敏）
- SSH tunnel 连接（通过 driver dial hook）
- SSH 代理复用（多个 profile 共享同一 SSH 配置）
- Keyring 密钥管理
- YAML 配置文件 + profile
- 多种输出格式：JSON、YAML、Table、CSV

## 快速开始

### 安装

```bash
# macOS (Homebrew)
brew install zx06/tap/xsql

# Windows (Scoop)
scoop bucket add zx06 https://github.com/zx06/scoop-bucket
scoop install xsql

# 或者使用 Go
go install github.com/zx06/xsql/cmd/xsql@latest

# 或者下载预编译二进制
# https://github.com/zx06/xsql/releases
```

### 配置

```bash
# 创建配置文件 ~/.config/xsql/xsql.yaml
mkdir -p ~/.config/xsql
cat > ~/.config/xsql/xsql.yaml << 'EOF'
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    password: "keyring:xsql/dev/password"  # 或明文（需 --allow-plaintext）
    database: test
EOF

# 执行查询
xsql query "SELECT 1" --profile dev --format json
```

### 输出示例

```bash
# JSON 格式（默认，适合 AI/程序）
$ xsql query "SELECT id, name FROM users" --profile dev -f json
{"ok":true,"schema_version":1,"data":{"columns":["id","name"],"rows":[{"id":1,"name":"Alice"}]}}

# Table 格式（终端友好）
$ xsql query "SELECT id, name FROM users" --profile dev -f table
id      name
----    ------
1       Alice

(1 rows)

# CSV 格式（数据导出）
$ xsql query "SELECT id, name FROM users" --profile dev -f csv
id,name
1,Alice
```

### 通过 SSH tunnel 连接

```yaml
# 定义可复用的 SSH 代理
ssh_proxies:
  bastion:
    host: jump.example.com
    user: admin
    identity_file: ~/.ssh/id_ed25519

profiles:
  prod:
    db: pg
    host: db.internal           # 数据库内网地址
    port: 5432
    user: app
    password: "keyring:xsql/prod/password"
    database: mydb
    ssh_proxy: bastion          # 引用预定义的 SSH 代理

  # 多个 profile 可以共享同一个 SSH 代理
  analytics:
    db: pg
    host: analytics.internal
    port: 5432
    user: readonly
    password: "keyring:xsql/analytics/password"
    database: analytics
    ssh_proxy: bastion          # 复用同一个代理
```

## 输出格式

| 格式 | 用途 | 元数据 |
|------|------|--------|
| `json` | AI/程序消费 | 包含 ok/schema_version |
| `yaml` | 人类阅读/配置 | 包含 ok/schema_version |
| `table` | 终端人类阅读 | 不包含，直接显示数据 |
| `csv` | 数据导出/表格 | 不包含，直接显示数据 |
| `auto` | 自动选择 | TTY→table，否则→json |

### 机器输出契约

```json
// 成功
{"ok":true,"schema_version":1,"data":{"columns":[...],"rows":[...]}}

// 失败
{"ok":false,"schema_version":1,"error":{"code":"XSQL_...","message":"...","details":{...}}}
```

## 文档索引
- 设计总览：`docs/architecture.md`
- CLI 约定与输出/错误规范：`docs/cli-spec.md`
- 输出与错误契约：`docs/error-contract.md`
- 配置与 Profile/Secret：`docs/config.md`
- 环境变量约定：`docs/env.md`
- SSH Proxy：`docs/ssh-proxy.md`
- 数据库驱动与只读策略：`docs/db.md`
- 开发指南：`docs/dev.md`
- AI-first 约定：`docs/ai.md`
- 设计变更（RFC）：`docs/rfcs/README.md`

# CLI 规范（AI-first）

## 设计目标
- 机器可读：默认在非 TTY 输出 JSON；错误也结构化。
- 稳定：字段名、错误码、退出码保持兼容。

## 输出约定
- 契约总览见：`docs/error-contract.md`
- 成功：`{"ok":true,"schema_version":1,"data":{...}}`
- 失败：`{"ok":false,"schema_version":1,"error":{"code":"...","message":"...","details":{...}}}`

## 退出码
| 退出码 | 含义 |
|--------|------|
| 0 | 成功 |
| 2 | 参数/配置错误 |
| 3 | 连接错误 |
| 4 | 只读策略拦截写入 |
| 5 | DB 执行错误 |
| 10 | 内部错误 |

## 命令

### `xsql query <SQL>`

执行 SQL 查询。

**默认只读模式**：为防止误操作，默认启用只读保护。如需执行写操作，使用 `--unsafe-allow-write` 标志。

**只读保护机制（双重保护）：**
1. **SQL 静态分析**：客户端检测 INSERT/UPDATE/DELETE/DROP 等写操作关键词
2. **数据库事务级只读**：使用 `BEGIN READ ONLY` 事务执行查询，数据库层面阻止任何写操作

```bash
# 基本用法（只读）
xsql query "SELECT * FROM users LIMIT 10" --profile dev

# 输出 JSON
xsql query "SELECT id, name FROM users" --profile dev --format json

# 允许写操作
xsql query "INSERT INTO logs (msg) VALUES ('test')" --profile dev --unsafe-allow-write
```

**Flags:**
| Flag | 默认值 | 说明 |
|------|--------|------|
| `--profile` | - | Profile 名称 |
| `--format` | auto | 输出格式：json/yaml/table/csv/auto |
| `--unsafe-allow-write` | false | 允许写操作（绕过只读保护） |
| `--allow-plaintext` | false | 允许配置中使用明文密码（也可在配置文件中设置 `allow_plaintext: true`） |
| `--ssh-skip-known-hosts-check` | false | 跳过 SSH 主机密钥验证（危险） |

**输出示例（JSON）：**
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

**输出示例（Table）：**
```
id      name
----    ------
1       Alice
2       Bob

(2 rows)
```

**输出示例（CSV）：**
```csv
id,name
1,Alice
2,Bob
```

> 注：Table 和 CSV 格式不包含 `ok` 和 `schema_version` 元数据，直接输出数据。

### `xsql schema dump`

导出数据库结构（表、列、索引、外键），供 AI/agent 自动理解数据库 schema。

```bash
# 导出所有表结构
xsql schema dump -p dev

# 输出 JSON 格式
xsql schema dump -p dev -f json

# 过滤特定表（支持通配符）
xsql schema dump -p dev --table "user*"

# 包含系统表
xsql schema dump -p dev --include-system
```

**Flags:**
| Flag | 默认值 | 说明 |
|------|--------|------|
| `--profile` | - | Profile 名称 |
| `--format` | auto | 输出格式：json/yaml/table/auto |
| `--table` | "" | 表名过滤（支持 `*` 和 `?` 通配符） |
| `--include-system` | false | 包含系统表 |
| `--allow-plaintext` | false | 允许配置中使用明文密码 |
| `--ssh-skip-known-hosts-check` | false | 跳过 SSH 主机密钥验证（危险） |

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "database": "mydb",
    "tables": [
      {
        "schema": "public",
        "name": "users",
        "comment": "用户表",
        "columns": [
          {
            "name": "id",
            "type": "bigint",
            "nullable": false,
            "default": "nextval('users_id_seq'::regclass)",
            "comment": "主键",
            "primary_key": true
          },
          {
            "name": "email",
            "type": "varchar(255)",
            "nullable": false,
            "default": null,
            "comment": "邮箱",
            "primary_key": false
          }
        ],
        "indexes": [
          {
            "name": "users_pkey",
            "columns": ["id"],
            "unique": true,
            "primary": true
          }
        ],
        "foreign_keys": []
      }
    ]
  }
}
```

**输出示例（Table）：**
```
Database: mydb

Table: public.users (用户表)
  Columns:
    name    type          nullable  default                   comment  pk
    ----    ----          --------  -------                   -------  --
    id      bigint        false     nextval('users_id_seq')   主键     ✓
    email   varchar(255)  false     -                         邮箱     

(1 table)
```

**使用场景：**
- AI 自动发现数据库结构，无需人工提供表信息
- 生成数据库文档
- 对比不同环境的 schema 差异

> **注意**：schema dump 是只读操作，遵循 profile 的只读策略。

### `xsql serve`

启动本地 Web 管理服务，提供 profile 浏览、schema 浏览和只读 SQL 查询能力。

```bash
# 启动服务
xsql serve

# 监听自定义地址
xsql serve --addr 127.0.0.1:9000

# 远程监听（必须提供 token）
xsql serve --addr 0.0.0.0:8788 --auth-token "your-token"
```

**Flags:**
| Flag | 默认值 | 说明 |
|------|--------|------|
| `--addr` | `127.0.0.1:8788` | Web 服务监听地址 |
| `--auth-token` | - | Bearer token；非 loopback 地址必填 |
| `--allow-plaintext` | false | 允许配置中使用明文 secrets |
| `--ssh-skip-known-hosts-check` | false | 跳过 SSH 主机密钥验证（危险） |

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "addr": "127.0.0.1:8788",
    "url": "http://127.0.0.1:8788/",
    "auth_required": false,
    "mode": "serve"
  }
}
```

### `xsql web`

启动 Web 管理服务，并在服务就绪后尝试打开默认浏览器。

```bash
xsql web
xsql web --addr 127.0.0.1:9000
```

> **注意**：浏览器打开为 best-effort；若打开失败，服务仍继续运行。

### Web API

Web UI 使用以下 HTTP API，统一返回 JSON Envelope：

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/v1/health` | 服务健康检查 |
| `GET` | `/api/v1/profiles` | 列出 profiles |
| `GET` | `/api/v1/profiles/{name}` | 查看 profile 详情（脱敏） |
| `GET` | `/api/v1/schema/tables` | 列出表（支持 `profile`/`table`/`include_system`） |
| `GET` | `/api/v1/schema/tables/{schema}/{table}` | 查看单表结构（支持 `profile`） |
| `POST` | `/api/v1/query` | 执行只读 SQL 查询 |

Web 查询强制只读，即使 profile 配置了 `unsafe_allow_write: true`，也不会在 Web 接口中生效。

### `xsql spec`

导出 tool spec（供 AI/agent 自动发现）。

```bash
xsql spec --format json
xsql spec --format yaml
```

> **注意**：`spec` 命令支持所有输出格式（`json`/`yaml`/`table`/`csv`/`auto`），但通常使用 `json` 或 `yaml` 供 AI 消费。

### `xsql version`

输出版本信息。

```bash
xsql version --format json
```

### `xsql profile list`

列出所有配置的 profiles。

```bash
xsql profile list --format json
```

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "config_path": "/home/user/.config/xsql/xsql.yaml",
    "profiles": [
      {"name": "dev", "description": "开发环境数据库", "db": "mysql", "mode": "read-only"},
      {"name": "prod", "description": "生产环境数据库", "db": "pg", "mode": "read-only"}
    ]
  }
}
```

### `xsql profile show <name>`

查看指定 profile 的详情（密码会被脱敏）。

```bash
xsql profile show dev --format json
```

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "config_path": "/home/user/.config/xsql/xsql.yaml",
    "name": "dev",
    "description": "开发环境数据库",
    "db": "mysql",
    "host": "localhost",
    "port": 3306,
    "user": "root",
    "database": "mydb",
    "unsafe_allow_write": false,
    "allow_plaintext": false,
    "dsn": "***",
    "password": "***",
    "ssh_proxy": "bastion",
    "ssh_host": "bastion.example.com",
    "ssh_port": 22,
    "ssh_user": "admin",
    "ssh_identity_file": "~/.ssh/id_ed25519"
  }
}
```

> 注：`dsn`/`password`/`ssh_*` 字段仅在对应配置存在时返回，并且会被脱敏。

### `xsql proxy`

启动端口转发代理，将本地端口通过 SSH tunnel 转发到指定 profile 的数据库。这类似于 `ssh -L` 的功能，但使用 xsql 的配置和 profile 系统。

```bash
# 使用 profile 启动代理（端口自动分配）
xsql proxy -p prod-mysql

# 指定本地端口
xsql -p prod-mysql proxy --local-port 13306

# 使用 profile 中配置的 local_port（如果设置了的话）
xsql proxy -p prod-mysql   # 自动使用 profile 中的 local_port

# 输出 JSON 格式
xsql -p prod-mysql proxy --format json
```

**端口优先级：**`--local-port` flag > `profile.local_port` > 0（自动分配）

**端口冲突处理：**
- 当端口来源于**配置文件**且端口已被占用时：
  - TTY 环境：交互式询问用户选择随机端口或退出
  - 非 TTY 环境：返回错误 `XSQL_PORT_IN_USE`
- 当端口来源于 `--local-port` CLI flag 且被占用时：直接返回错误

**要求：**
- Profile 必须配置 `ssh_proxy`，否则无法启动
- Profile 必须配置数据库类型（`db`）、主机（`host`）和端口（`port`）

**Flags:**
| Flag | 默认值 | 说明 |
|------|--------|------|
| `--local-port` | 0 | 本地监听端口（0 表示自动分配） |
| `--local-host` | 127.0.0.1 | 本地监听地址 |
| `--allow-plaintext` | false | 允许配置中使用明文密码 |
| `--ssh-skip-known-hosts-check` | false | 跳过 SSH 主机密钥验证（危险） |

**全局 Flags:**
| Flag | 说明 |
|------|------|
| `-p, --profile <name>` | Profile 名称（必需） |

**输出示例（Table）：**
```
✓ Proxy started successfully
  Local:   127.0.0.1:13306
  Remote:  db.internal.example.com:3306 (via bastion.example.com:22)
  Profile: prod-mysql

Press Ctrl+C to stop
```

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "local_address": "127.0.0.1:13306",
    "remote_address": "db.internal.example.com:3306",
    "ssh_proxy": "bastion.example.com:22",
    "profile": "prod-mysql"
  }
}
```

**使用场景：**
- 本地开发时需要通过 SSH tunnel 访问远程数据库
- 让本地数据库客户端（如 DBeaver、DataGrip）或 IDE 直接连接远程数据库
- 替代手动执行 `ssh -L` 命令

**安全说明：**
- 默认监听 `127.0.0.1`，仅本地可访问
- 复用 SSH 配置的 known_hosts 校验
- 密码/passphrase 复用 keyring 机制，不泄露明文
- 按 Ctrl+C 或发送 SIGTERM 信号优雅关闭代理

### `xsql config init`

创建配置文件模板。

```bash
# 在默认路径创建
xsql config init

# 指定路径
xsql config init --path ./xsql.yaml
```

**Flags:**
| Flag | 默认值 | 说明 |
|------|--------|------|
| `--path` | `$HOME/.config/xsql/xsql.yaml` | 配置文件路径 |

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "config_path": "/home/user/.config/xsql/xsql.yaml"
  }
}
```

### `xsql config set <key> <value>`

快速修改配置项，使用点号分隔的路径定位配置字段。

```bash
# 设置 profile 字段
xsql config set profile.dev.host localhost
xsql config set profile.dev.port 3306
xsql config set profile.dev.db mysql
xsql config set profile.dev.local_port 13306

# 设置 SSH proxy 字段
xsql config set ssh_proxy.bastion.host bastion.example.com
xsql config set ssh_proxy.bastion.user admin
```

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "config_path": "/home/user/.config/xsql/xsql.yaml",
    "key": "profile.dev.host",
    "value": "localhost"
  }
}
```

## 全局 Flags

| Flag | 说明 |
|------|------|
| `--config <path>` | 指定 YAML 配置文件路径 |
| `--profile <name>` | 选择 profile（等价 ENV：`XSQL_PROFILE`） |
| `--format <fmt>` | 输出格式（等价 ENV：`XSQL_FORMAT`） |

## 格式说明

| 格式 | 用途 | 元数据 |
|------|------|--------|
| `json` | AI/程序消费 | 包含 ok/schema_version |
| `yaml` | 人类阅读/配置 | 包含 ok/schema_version |
| `table` | 终端人类阅读 | 不包含，直接显示数据 |
| `csv` | 数据导出/表格 | 不包含，直接显示数据 |
| `auto` | 自动选择 | TTY→table，否则→json |

### `xsql mcp server`

启动 MCP (Model Context Protocol) server，提供数据库查询能力给 AI 助手。

```bash
# 启动 MCP server（使用 stdio 传输）
xsql mcp server

# 指定配置文件
xsql mcp server --config /path/to/config.yaml

# 使用 Streamable HTTP 传输（必须提供鉴权 token）
xsql mcp server --transport streamable_http --http-addr 127.0.0.1:8787 --http-auth-token "your-token"
```

**参数：**

| 参数 | 说明 |
|------|------|
| `--transport` | MCP 传输方式：`stdio`（默认）或 `streamable_http` |
| `--http-addr` | Streamable HTTP 监听地址（默认 `127.0.0.1:8787`） |
| `--http-auth-token` | Streamable HTTP 鉴权 token（仅 `streamable_http` 必填） |

**MCP Tools:**

1. **query**: 执行 SQL 查询
   ```json
   {
     "name": "query",
     "description": "Execute SQL query on database",
     "inputSchema": {
       "type": "object",
       "properties": {
         "sql": {"type": "string", "description": "SQL query to execute"},
         "profile": {"type": "string", "description": "Profile name to use"}
       },
       "required": ["sql", "profile"]
     }
   }
   ```

2. **profile_list**: 列出所有 profiles
   ```json
   {
     "name": "profile_list",
     "description": "List all configured profiles",
     "inputSchema": {"type": "object", "properties": {}}
   }
   ```

3. **profile_show**: 查看 profile 详情
   ```json
   {
     "name": "profile_show",
     "description": "Show profile details",
     "inputSchema": {
       "type": "object",
       "properties": {
         "name": {"type": "string", "description": "Profile name"}
       },
       "required": ["name"]
     }
   }
   ```

**输出格式：** MCP tool 调用返回 JSON 格式，遵循 xsql 的标准输出契约（`ok`、`schema_version`、`data`/`error`）。

**安全说明：**
- query tool 默认只读模式（双重保护：SQL 静态分析 + DB 事务级只读）
- 写操作需要显式设置 `unsafe_allow_write: true`
- Streamable HTTP 传输要求鉴权，请在请求中提供 `Authorization: Bearer <token>` 头

## 参数来源优先级
- CLI > ENV > Config

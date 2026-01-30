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

### `xsql spec`

导出 tool spec（供 AI/agent 自动发现）。

```bash
xsql spec --format json
xsql spec --format yaml
```

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
    "password": "***",
    "database": "mydb"
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
```

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

## 参数来源优先级
- CLI > ENV > Config

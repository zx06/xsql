# AI-first 约定

## 目标
让 LLM/agent 可以可靠调用：
- 输出可预测、可机读
- 错误码稳定
- 命令与参数可被自动发现（tool spec）
- 自动发现数据库结构（schema dump）

## 规范建议
- 非 TTY 默认输出 JSON；TTY 默认 table。
- 错误对象：`code/message/details`；并保证退出码与 code 对应。
- 提供 `xsql spec --format json` 导出：
  - commands/flags/env mapping
  - output schema
  - error codes
- 提供 `xsql schema dump` 导出数据库结构：
  - 表名、列名、类型、约束
  - 索引、外键关系
  - 供 AI 自动理解数据库结构

## 兼容性
- 对 JSON 输出字段做版本化（`schema_version`），新字段只增不改；详细契约见 `docs/error-contract.md`。

## Schema 发现

AI 可以通过 `xsql schema dump` 自动发现数据库结构：

```bash
# 导出所有表结构（JSON 格式）
xsql schema dump -p dev -f json

# 过滤特定表
xsql schema dump -p dev --table "user*" -f json

# 输出示例
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "database": "mydb",
    "tables": [
      {
        "schema": "public",
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

**AI 工作流建议：**
1. 先调用 `xsql schema dump` 获取表结构
2. 理解表名、列名、类型、关系
3. 基于结构生成正确的 SQL 查询
4. 调用 `xsql query` 执行查询

## MCP Server
xsql 提供了 MCP (Model Context Protocol) Server 模式，允许 AI 助手通过标准 MCP 协议访问数据库查询能力。

### 启动方式
```bash
xsql mcp server
```

### Streamable HTTP 传输
需要通过 `streamable_http` 启动，并强制要求鉴权：
```bash
xsql mcp server --transport streamable_http --http-addr 127.0.0.1:8787 --http-auth-token "your-token"
```

### MCP Tools
MCP Server 提供以下 tools：
- **query**: 执行 SQL 查询（支持只读模式）
- **profile_list**: 列出所有配置的 profiles
- **profile_show**: 查看 profile 详情
- **schema_dump**: 导出数据库结构（表、列、索引、外键）

### 集成示例
在 Claude Desktop 配置中添加：
```json
{
  "mcpServers": {
    "xsql": {
      "command": "xsql",
      "args": ["mcp", "server", "--config", "/path/to/config.yaml"]
    }
  }
}
```

### 详细规范
详见 `docs/cli-spec.md` 中的 `xsql mcp server` 命令说明。

## Web UI
xsql 还提供本地 Web UI 模式，用于人工交互式查询和 schema 浏览：

```bash
xsql serve
xsql web
```

Web UI 复用 xsql 的 profile、SSH、只读策略和结构化错误契约，但其 HTTP API 面向浏览器，不等同于 MCP 协议。

## AI TUI 交互模式 (xsql-ai)
xsql-ai 为独立的 CLI 可执行程序，提供交互式 AI 终端模式。用户只需在终端以自然语言发问，AI 结合当前数据库 Schema 结构自动构建对应的 SQL 查询，并在 TUI 中提供交互预览与安全执行：

```bash
# 启动交互式 TUI
xsql-ai --profile dev
```

### LLM 集成与 Tool Call 机制
`xsql` 使用 OpenAI 官方 SDK (`github.com/openai/openai-go`) 与大模型交互，支持双 Tool Calling 与多轮数据集召回：
- 数据库查询 Tool：`execute_sql(sql: string, explanation: string)`
- JS 数据分析 Tool：`execute_javascript(js_code: string, explanation: string)`

#### 零数据泄露与 Session 数据集召回 (Session DataStore)
- 每次查询成功的结果在本地分配标号（`res1`, `res2`, ...）。
- 大模型上下文中仅包含数据集的轻量 Catalog 目录结构（字段名与行数），不传输海量真实数据。
- AI 可通过 `execute_javascript` 生成纯 Go 沙箱 (`goja`) 执行的代码，在本地对 `res1`, `res2` 等数据集做跨表 Join、占比统计与数据清洗，并通过 Go 宿主层安全导出为 CSV/JSON/Markdown。

### 快捷键操作

#### SQL 待确认状态 (SQL Preview Mode)
- `Enter`: 确认并安全执行当前生成预览的 SQL
- `e`: 切换到 SQL 文本手工编辑/微调模式
- `Esc`: 取消当前 SQL 生成建议，返回 Prompt 输入模式

#### 通用与表格操作 (General & Table Operations)
- `Enter`: 提交自然语言需求给 AI
- `Ctrl+E`: 展开/收起折叠全量内容 (Toggle Expanded Full View，无 50 行截断)
- `Tab`: 在历史多个查询结果表格之间无缝切换焦点 (`[FOCUSED]`)
- `←` / `→`: 横向平滑滚动查看当前焦点表格的隐藏列
- `PgUp` / `PgDn`: 向上/向下翻页查看当前焦点表格的第 13-N 行数据
- `Shift+Tab`: 一键切换 **自动执行 (AUTO-EXECUTE)** 与 **手动批准 (MANUAL-APPROVE)** 模式
- `Esc` / `Ctrl+C`: 退出 AI 模式



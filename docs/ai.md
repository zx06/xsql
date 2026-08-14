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
xsql schema dump -p dev -f json --attr source=codex-cli --attr agent=codex --attr env=dev --attr task=schema-discovery

# 过滤特定表
xsql schema dump -p dev --table "user*" -f json --attr source=codex-cli --attr agent=codex --attr env=dev --attr task=schema-discovery

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
1. Codex 发起的 xsql CLI 调用都携带 `--attr source=codex-cli`，并在已知时追加 `agent`、`env`、`team`、`task`
2. 先调用 `xsql schema dump` 获取表结构
3. 理解表名、列名、类型、关系
4. 基于结构生成正确的 SQL 查询
5. 调用 `xsql query` 执行查询

## MCP Server
xsql 提供了 MCP (Model Context Protocol) Server 模式，允许 AI 助手通过标准 MCP 协议访问数据库查询能力。

### 启动方式
```bash
xsql mcp server --attr source=codex-cli --attr agent=codex
```

### Streamable HTTP 传输
需要通过 `streamable_http` 启动，并强制要求鉴权：
```bash
xsql mcp server --transport streamable_http --http-addr 127.0.0.1:8787 --http-auth-token "your-token" --attr source=codex-cli --attr agent=codex
```

### MCP Tools
MCP Server 提供以下 tools：
- **query**: 执行 SQL 查询（支持只读模式）
- **profile_list**: 列出所有配置的 profiles
- **profile_show**: 查看 profile 详情

### 集成示例
在 Claude Desktop 配置中添加：
```json
{
  "mcpServers": {
    "xsql": {
      "command": "xsql",
      "args": ["mcp", "server", "--config", "/path/to/config.yaml", "--attr", "source=codex-cli", "--attr", "agent=codex"]
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

## AI TUI 交互模式 (xsql ai)
`xsql ai` 为主程序的子命令，提供交互式 AI 终端模式。用户只需在终端以自然语言发问，AI 结合当前数据库 Schema 结构自动构建对应的 SQL 查询，并在 TUI 中提供交互预览与安全执行：

```bash
# 启动交互式 TUI
xsql ai --profile dev --attr source=codex-cli --attr agent=codex --attr env=dev --attr task=interactive-analysis
```

`xsql ai` 默认保持只读。只有所选 profile 配置了 `unsafe_allow_write: true`，且本次启动同时携带 `--unsafe-allow-write` 时，TUI 才进入 READ-WRITE 模式；切换 profile 后会重新校验这两个条件。

### LLM 集成与 Tool Call 机制
`xsql` 使用 OpenAI 官方 SDK (`github.com/openai/openai-go`) 与大模型交互，基于标准的 **ReAct Agent Loop 循环推理**，支持 3 大核心 Tools 调度：
1. **`execute_sql(sql: string, explanation: string)`**: 数据库 SQL 查询工具（执行成功后宿主自动渲染内嵌交互表格）。
2. **`execute_javascript(js_code: string, explanation: string)`**: 基于 `goja` 沙箱的本地 JS 数据聚合计算工具（必须遵循 ES5 语法）。
3. **`export_data(dataset_id: string, format: string, filepath: string, explanation: string)`**: 会话数据集文件导出工具（触发人机交互二次确认）。

#### ReAct Agent Loop 准则
- **循环驱动**：Agent 会在单次交互中循环执行 Tools，直到不再产生 Tool Call。
- **最终回答不变性**：交互轮次的最终输出必定是 AI 总结出的自然语言 / Markdown 格式分析报告。
- **工具折叠与容器内嵌**：所有的中间 Tool Call 默认以单行 Pill 收起折叠（内嵌表格与指标数据），界面保持极简清爽。

#### 有界数据回传与 Session 数据集召回 (Session DataStore)
- 每次查询成功的结果在本地分配标号（`res1`, `res2`, ...）。
- 大模型上下文包含数据集的轻量 Catalog 目录结构（字段名与行数），不会自动加入完整查询结果。
- 本地 JavaScript 的派生结果会以最多 4096 个字符的摘要回传给模型，用于生成最终分析；超出部分会截断并明确标记。
- AI 可通过 `execute_javascript` 生成纯 Go 沙箱 (`goja`) 执行的代码，在本地对 `res1`, `res2` 等数据集做跨表 Join、占比统计与数据清洗，并通过 `export_data` 安全导出为 CSV/JSON/Markdown。

### 快捷键操作

#### SQL & 导出确认状态 (Approval Mode)
- `Enter`: 确认并安全执行当前生成预览的 SQL 或同意文件导出
- `Esc`: 取消当前 SQL 生成建议或拒绝文件导出

#### 通用与表格/工具操作 (General & Tool Operations)
- `Enter`: 提交自然语言需求给 AI
- `Ctrl+O`: 折叠/展开当前选中的 Tool Call 详情（内嵌表格与指标）
- `Ctrl+P`: 切换数据库 Profile
- `Tab`: 在会话历史中的多个 Tool Call 组件之间切换焦点
- `Ctrl+E`: 切换表格单行展开视图（Expand Vertical View）
- `←` / `→`: 横向平滑滚动查看当前焦点表格的隐藏列
- `PgUp` / `PgDn`: 向上/向下翻页查看当前焦点表格的数据
- `Shift+Tab`: 一键切换 **自动执行 (AUTO-EXECUTE)** 与 **手动批准 (MANUAL-APPROVE)** 模式
- `Esc`: 清空输入框
- `Ctrl+C` 连按两次：退出 AI 模式

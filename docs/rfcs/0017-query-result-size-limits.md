# RFC 0017: Query result size limits

Status: Accepted

## 摘要

为 `xsql query`（CLI / Web / MCP / TUI）增加默认行数上限，在数据库驱动层扫描时提前停止，避免大结果集 OOM 与 MCP/agent 上下文膨胀。截断时在成功响应中附加 `truncated`、`row_count`、`max_rows` 与 agent 提示。CLI/Web 允许 `--max-rows 0` 显式取消上限；MCP 保持强制上限。顺带修复 schema 工具的空 `tables` 与 `include_system` 行为。

## 背景 / 动机

- `internal/db/query.go` 的 `scanRows` 无上限，整表 `SELECT *` 可拖垮进程。
- MCP 将完整 JSON 放入 tool 文本，成本高于 TUI 的 `BoundToolFeedback`。
- PR #82 后 MCP query 与 CLI 共用 `app.Query`，可在单一路径扩展。

## 方案（Proposed）

### 默认与优先级

| 场景 | 默认上限 | 取消上限 |
|------|----------|----------|
| CLI / Web / TUI | 1000 行 | `--max-rows 0` 或 API `max_rows: 0`（须显式） |
| MCP | 500 行 | 不支持无上限；`max_rows: 0` 视为未设置，走默认链 |

MCP 硬上限：**`mcp.max_rows_limit`（默认 10000）**，超过则 clamp 并输出 `max_rows_requested` / `max_rows_limit`。

优先级：**CLI/API 参数 > ENV (`XSQL_MAX_ROWS`) > profile `max_rows` > MCP `mcp.max_rows`（仅 MCP）> 上表默认值**。

选用 1000：足够日常探查，且单行 JSON 通常仍可控；MCP 500 更保守，减少 tool 响应 token。

### 输出（`schema_version` 仍为 1）

成功时 `data` 在既有 `columns`/`rows` 上**只增**字段：

```json
{
  "truncated": true,
  "row_count": 1000,
  "max_rows": 1000,
  "truncated_reason": "max_rows",
  "hint": "Result truncated by max_rows; add SQL LIMIT/OFFSET, use aggregation, or run MCP `schema_list_tables` or `xsql schema dump` for schema discovery."
}
```

字节护栏截断时追加 `truncated_reason: "max_response_bytes"`（可与 `max_rows` 并存）、`max_response_bytes`，且 **`max_rows` 始终为生效的配置行数上限**（不是当前行数）。`hint` 与 `truncated_reason` 一致（两种原因均出现时 hint 合并两句）。

未截断时不输出 `truncated`/`hint`（`row_count` 可选，与现有客户端兼容）。

### CSV

stdout 仍为纯 CSV；若截断，在 **stderr** 输出一行 `xsql: query result truncated (...)` + hint，**退出码仍为 0**。

### MCP 响应字节护栏（第二层）

配置 `mcp.max_response_bytes`（默认 524288）。序列化后若超限，用二分查找适配行数；metadata 见上。

### Schema 修复

- 空库 `schema_list_tables` 的 `tables` 统一为 `[]`。
- `include_system: true` 时包含 MySQL 系统库表与 PostgreSQL `pg_catalog` / `information_schema` 等（与 RFC 0005 一致）。

## 兼容性与迁移

- 破坏性：默认新增 1000 行 cap；依赖全量无 LIMIT 查询的脚本需 `--max-rows 0` 或调大 profile。
- `schema_version` 不变。

## 测试计划

- 单元：`ResolveQueryMaxRows`、`scanRows` 提前停止（sqlmock）、MCP 字节裁剪。
- 集成 / E2E：MySQL 8.0、PostgreSQL 16 验证默认、自定义、无限、截断元数据。

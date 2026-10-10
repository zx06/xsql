# RFC 0015: MCP 复用 app 层与 Schema Discovery Tools

Status: Implemented

## 摘要
MCP Server 的 `query` 工具此前在 `internal/mcp` 内重复实现连接/SSH 逻辑，未复用 `internal/app`，导致 profile 的 `skip_host_key` 等行为与 CLI/Web 不一致。本 RFC 将 MCP 数据库操作统一经 `internal/app`（与 Web UI 相同），并新增轻量 schema discovery tools，使 AI 客户端无需猜测表名。

## 背景 / 动机
- **痛点**：MCP 约 150 行重复连接代码；SSH `skip_host_key` 在 MCP 路径被忽略；无 schema 工具，agent 只能手写 information_schema 查询。
- **目标**：MCP/Web/CLI 共享 `app.ResolveProfile` / `app.Query` / `app.ListTables` 等；暴露与 CLI JSON 契约一致的 schema tools；保留 RFC 0014 写入双重授权与 stats 记录。
- **非目标**：MCP query 默认行数限制（后续 PR）；MCP 侧 `--ssh-skip-known-hosts-check` CLI 开关（仍仅 profile `skip_host_key` + 与 CLI 相同的 app 解析逻辑）。

## 方案（Proposed）
### MCP Tools（新增）
| Tool | 用途 | 输入 |
|------|------|------|
| `schema_list_tables` | 轻量表清单（LLM 友好） | `profile`, 可选 `table_pattern`, `include_system` |
| `schema_dump` | 与 `xsql schema dump` 等价的结构化导出 | `profile`, 可选 `table_pattern`, `include_system` |
| `schema_describe_table` | 单表列/索引/外键详情 | `profile`, `table`, 可选 `schema`（PG 必填） |

输出均遵循标准信封：`{"ok":true,"schema_version":1,"data":{...}}`。

### 技术设计
- `internal/mcp`：`query` / schema tools 调用 `internal/app`；`WriteAllowed` 不变；stats `cmd` 字段区分 `query`、`schema_list_tables`、`schema_dump`、`schema_describe_table`。
- 导出 `app.SSHOptionsForProfile`（原 `resolveSSHOptions` 的公开包装）供一致性单测。
- `query` 路径保留 `app.QueryRequest` 结构，便于后续扩展默认 `LIMIT` 等选项。

## 兼容性与迁移
- **不破坏**：现有 tool 名称与 query/profile 工具 JSON 契约不变；新增 tools 为纯增量。
- query 错误文案可能与 CLI 完全一致（如 `profile not found`），不再使用 MCP 专有 wording。

## 安全与隐私
- schema tools 为只读；不暴露 secrets。
- 写入仍仅 `query` + 双重授权（RFC 0014）。

## 测试计划
- 单元：`SSHOptionsForProfile` profile `skip_host_key`；MCP handler 参数校验。
- E2E：MCP schema tools 对 MySQL 8 / PostgreSQL 16；SSH skip_host_key 与 CLI 一致（dial 阶段错误码）。

## 未决问题
- 无

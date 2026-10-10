# RFC 0014: MCP 写入双重授权

Status: Implemented

## 摘要
MCP `query` tool 此前仅检查 profile 的 `unsafe_allow_write`，在 profile 开启写入后 AI 客户端可直接执行 DML/DDL，与 CLI/TUI 的「profile + 运行时开关」双重授权不一致。本 RFC 要求 MCP 写入同样满足 profile 与 MCP server 启动参数双条件，默认保持只读，读查询行为不变。

## 背景 / 动机
- 当前痛点：`internal/mcp/tools.go` 将 `UnsafeAllowWrite` 直接设为 `profile.UnsafeAllowWrite`，绕过第二因子。
- 目标：与 `AGENTS.md`、`docs/cli-spec.md` 及 xsql skill 中的安全承诺一致。
- 非目标：不改变 Web UI（仍强制只读）；不新增 MCP 专用 profile 字段；不要求每次 tool call 额外参数（server 启动参数作为 MCP 的「本次会话」第二因子）。

## 方案（Proposed）
### 用户视角（CLI/配置）
- `xsql mcp server` 新增 flag：`--unsafe-allow-write`（默认 `false`）。
- ENV：`XSQL_MCP_UNSAFE_ALLOW_WRITE`（`true`/`1` 为启用）；优先级：**CLI flag > ENV**（与 transport 等 MCP server 选项一致）。
- 写入条件（与 `xsql query` 对称）：
  1. profile `unsafe_allow_write: true`
  2. MCP server 启动时启用 `--unsafe-allow-write`（或对应 ENV）
- 任一缺失时，写 SQL 仍由只读策略拦截，返回 `XSQL_RO_BLOCKED`。

### 技术设计（Architecture）
- `internal/mcp.WriteAllowed(serverAllows, profileAllows bool) bool`
- `ToolHandler` 持有 `serverUnsafeAllowWrite`，由 `CreateServer(..., ServerOptions)` 注入。
- `query` tool 调用 `db.Query` 时：`UnsafeAllowWrite: WriteAllowed(h.serverUnsafeAllowWrite, profile.UnsafeAllowWrite)`。

### 兼容性
- **破坏行为变更（安全修复）**：此前仅 profile 开启即可经 MCP 写入；修复后还需 server 启动参数。读查询无变化。
- Claude Desktop / Cursor 的 MCP 配置需在需要写入时显式追加 `args` 中的 `--unsafe-allow-write`。

## 安全与隐私
- 默认 MCP server 只读；Streamable HTTP 鉴权要求不变。
- 不在 tool 输出中暴露 server 是否启用写入（profile_show 仍仅反映 profile 能力）。

## 测试计划
- 单元测试：`WriteAllowed` 真值表；`resolveMCPServerOptions` 对 flag/ENV 的解析。
- E2E：`profile unsafe_allow_write` + 无 server flag → `XSQL_RO_BLOCKED`；双开时写操作不再被只读层拦截（可仍因 SQL/DB 失败）。

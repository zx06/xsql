# RFC 0015: AI Agent 错误指引（Error Guidance）

Status: Accepted

## 摘要
为 xsql 的 JSON/YAML 错误 `details` 增加面向 AI/agent 的稳定指引字段（`hint`、`retryable`、`category`、方言与原生 DB 错误码），在不改变顶层 `error.code`、退出码与 `schema_version` 的前提下，减少 agent 对 DB/连接/只读类错误的无效重试。

## 背景 / 动机
- 当前痛点：`XSQL_DB_EXEC_FAILED` 等错误仅携带驱动原始 message，agent 难以判断应改 SQL、查 schema、修连接还是开启写入。
- 目标：在 `error.details` 中追加可机读、可行动的指引；MySQL errno 与 PostgreSQL SQLSTATE 映射到少量稳定 `category`。
- 非目标：不改变错误码含义、不提升 `schema_version`、不在 details 中暴露密码/完整 DSN。

## 方案（Proposed）
### 用户视角（CLI/配置/输出）
- 失败响应仍为 `{"ok":false,"schema_version":1,"error":{"code","message","details"}}`。
- `details` **只增**字段（已有键如 `reason` 保留）：
  - `hint` (string)：简短下一步建议（英文，便于模型直接遵循）。
  - `retryable` (bool)：是否建议在相同输入下立即重试。
  - `category` (string)：稳定语义分类（如 `unknown_table`、`syntax_error`、`read_only_blocked`）。
  - `dialect` (string, 可选)：`mysql` | `pg`。
  - `native_code` (string, 可选)：MySQL errno 或 PostgreSQL SQLSTATE。
  - `native_code_kind` (string, 可选)：`mysql_errno` | `pg_sqlstate`。

### 技术设计（Architecture）
- `internal/errors`：`New`/`Wrap` 返回前调用 `enrichDetails`，根据 `code`、`details` 与 `cause` 合并指引（不覆盖调用方已设置的 details 键）。
- `internal/errors/dbclassify.go`：解析 `go-sql-driver/mysql.MySQLError` 与 `pgconn.PgError`。
- `internal/db/query.go`：执行失败时在 details 传入 `dialect`。
- CLI / MCP / Web 共用 `internal/errors` 与 `internal/output`，无需各自 duplicate。

## 兼容性与迁移
- 不破坏兼容：仅追加 `details` 字段；`schema_version` 保持 `1`。
- 无迁移步骤。

## 测试计划
- 单元测试：MySQL/PG 典型 errno/SQLSTATE → category/hint/retryable。
- 集成测试（Docker MySQL 8.0 / PG 16）：未知表、语法错误、只读拦截产生预期 `hint`。

## 安全与隐私
- `hint`/`category` 不含 secrets；`native_code` 为公开错误码，非连接串。

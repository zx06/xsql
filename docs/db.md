# 数据库（MySQL / PostgreSQL）

## Driver 抽象（建议）
- driver registry：通过 `db.Register(name, driver)` 扩展。
- 统一连接解析为 `ResolvedConn`，避免上层感知方言细节。

## 只读（RO）策略
已确认：两者都做（DB 原生只读优先 + SQL 判定兜底）。

## SSH Proxy 与 driver dial
- `database/sql` 不提供通用替换 `net.Conn` 的入口；需要依赖各 driver 的 dial hook。
- 本项目优先采用 **driver 自定义 dial + sshClient.Dial**（详见 `docs/ssh-proxy.md`），并保留未来回退到本地端口转发的可能。

### DB 原生只读
- PostgreSQL：会话/事务只读（如 `SET default_transaction_read_only=on` 或 `BEGIN READ ONLY`）
- MySQL：`SET SESSION TRANSACTION READ ONLY` + `START TRANSACTION READ ONLY`

### SQL 判定兜底
- 优先 AST 判定（方言解析器）。
- 解析失败时采取**保守策略（默认拒绝写入）**：
  - 默认允许：`SELECT`、`WITH`（仅当最终语句为只读查询）、`EXPLAIN`、`SHOW`/`DESCRIBE`（视方言支持）
  - 默认拒绝：`INSERT/UPDATE/DELETE`、`CREATE/ALTER/DROP/TRUNCATE`、`GRANT/REVOKE`、`CALL/DO`、`COPY`（pg）等
- 提供 `--unsafe-allow-write` 逃生阀。

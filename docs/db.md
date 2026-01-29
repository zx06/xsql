# 数据库（MySQL / PostgreSQL）

## Driver 抽象
- driver registry：通过 `db.Register(name, driver)` 扩展。
- 统一连接配置为 `ConnOptions`，支持 DSN 或 host/port/user/password 字段。

## 只读（RO）策略

### 当前实现：SQL 判定
- 基于 SQL 关键字判定（非 AST 解析）
- 默认允许：`SELECT`、`WITH`、`EXPLAIN`、`SHOW`、`DESCRIBE`
- 默认拒绝：`INSERT/UPDATE/DELETE`、`CREATE/ALTER/DROP/TRUNCATE` 等
- 提供 `--unsafe-allow-write` 逃生阀

### 计划中：DB 原生只读
> 后续版本可能增加 DB 原生只读模式，作为额外安全层。

- PostgreSQL：会话/事务只读（如 `SET default_transaction_read_only=on`）
- MySQL：`SET SESSION TRANSACTION READ ONLY`

## SSH Proxy 与 driver dial
- `database/sql` 不提供通用替换 `net.Conn` 的入口；需要依赖各 driver 的 dial hook。
- 本项目采用 **driver 自定义 dial + sshClient.Dial**（详见 `docs/ssh-proxy.md`）。
- 保留未来回退到本地端口转发的可能。

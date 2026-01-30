# 数据库（MySQL / PostgreSQL）

## Driver 抽象
- driver registry：通过 `db.Register(name, driver)` 扩展。
- 统一连接配置为 `ConnOptions`，支持 DSN 或 host/port/user/password 字段。

## 只读（RO）策略

### 双重保护机制（默认启用）
xsql 采用双重保护防止误操作写数据库：

1. **SQL 静态分析**（客户端）
   - 基于 SQL 关键字判定（非 AST 解析）
   - 默认允许：`SELECT`、`WITH`、`EXPLAIN`、`SHOW`、`DESCRIBE`
   - 默认拒绝：`INSERT/UPDATE/DELETE`、`CREATE/ALTER/DROP/TRUNCATE` 等

2. **数据库事务级只读**（服务端）
   - PostgreSQL/MySQL：使用 `BEGIN READ ONLY` 事务执行查询
   - 数据库层面阻止任何写操作

### 写操作控制
- 默认：只读模式（双重保护都启用）
- 允许写操作：使用 `--unsafe-allow-write` 标志或配置 `unsafe_allow_write: true`
  - 绕过 SQL 静态分析检查
  - 绕过数据库事务级只读限制

## SSH Proxy 与 driver dial
- `database/sql` 不提供通用替换 `net.Conn` 的入口；需要依赖各 driver 的 dial hook。
- 本项目采用 **driver 自定义 dial + sshClient.Dial**（详见 `docs/ssh-proxy.md`）。
- 保留未来回退到本地端口转发的可能。

# 数据库（MySQL / PostgreSQL）

## Driver 抽象
- driver registry：通过 `db.Register(name, driver)` 扩展。
- 统一连接配置为 `ConnOptions`，支持 DSN 或 host/port/user/password 字段。

## 只读（RO）策略

xsql 采用**双重只读保护**机制确保数据安全：

1. **SQL 词法分析**（客户端层）：解析 SQL 语句，拦截写入操作
2. **数据库事务级只读**（服务端层）：使用 `sql.TxOptions{ReadOnly: true}` 强制只读事务

### 第一层：SQL 词法分析（客户端）

xsql 实现了完整的 SQL 词法分析器（`internal/db/readonly.go`），而非简单的字符串匹配：

#### 词法分析能力
- **注释处理**：支持 `--` 行注释和 `/* */` 块注释（包括嵌套注释）
- **字符串解析**：
  - 单引号字符串 `'...'`（MySQL/PostgreSQL）
  - 双引号字符串 `"..."`（PostgreSQL/ANSI）
  - 反引号标识符 `` `...` ``（MySQL）
  - 美元引号字符串 `$tag$...$tag$`（PostgreSQL）
- **转义处理**：正确处理 `''`、`""`、`` `` `` 等转义序列

#### 允许的首关键字（Allowlist）
以下 SQL 语句被视为安全的只读操作：
- `SELECT`：标准查询
- `SHOW` / `DESCRIBE` / `DESC`：MySQL 信息查询
- `EXPLAIN`：执行计划分析（包括 `EXPLAIN ANALYZE`）
- `WITH`：Common Table Expression（递归 CTE 也支持）
- `TABLE`：MySQL 8.0+ TABLE 语句
- `VALUES`：PostgreSQL VALUES 语句

#### 禁止的关键字（Denylist）
以下关键字在任何位置出现都会触发拦截：

**DML（数据操作）**：
- `INSERT`、`UPDATE`、`DELETE`、`MERGE`、`UPSERT`
- `REPLACE`（MySQL）
- `COPY`（PostgreSQL 批量操作）

**DDL（数据定义）**：
- `CREATE`、`ALTER`、`DROP`、`TRUNCATE`
- `GRANT`、`REVOKE`

**存储过程/函数**：
- `CALL`、`DO`、`EXECUTE`、`PREPARE`、`DEALLOCATE`

**事务控制（必须拦截以防止解除只读）**：
- `BEGIN`、`COMMIT`、`ROLLBACK`、`SAVEPOINT`、`RELEASE`
- `SET`：可能解除只读（如 `SET TRANSACTION READ WRITE`）

**锁定**：
- `LOCK`、`UNLOCK`、`LOAD`

#### 特殊检测：Data-Modifying CTE
PostgreSQL 支持在 CTE 中执行写入操作（`WITH ... DELETE/UPDATE/INSERT ... RETURNING ...`）。

xsql 会检测 CTE 内部是否包含写入关键字，例如：
```sql
-- 以下会被拦截
WITH deleted AS (DELETE FROM logs WHERE old = true RETURNING id)
SELECT * FROM deleted;

-- 纯读取 CTE 是允许的
WITH stats AS (SELECT user_id, COUNT(*) FROM orders GROUP BY user_id)
SELECT * FROM stats;
```

#### 绕过攻击防护

**多语句攻击**：
```sql
-- 会被拦截（包含多个有效语句）
SELECT 1; DELETE FROM users;
```

**注释混淆**：
```sql
-- 正确处理注释，以下会被允许（注释中的 DELETE 不影响）
SELECT /* DELETE */ 1;
```

**字符串伪装**：
```sql
-- 正确处理字符串，以下会被允许（字符串内容不影响判定）
SELECT 'DELETE FROM users' AS warning;
```

### 第二层：数据库事务级只读（服务端）

当 `unsafe_allow_write = false` 时，xsql 使用 Go 的 `database/sql` 包创建只读事务：

```go
tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
```

这意味着即使 SQL 静态分析被绕过（理论上），数据库层也会阻止任何写入操作。

### 写操作控制

#### 安全模式（默认）
```bash
# 默认启用双重只读保护
xsql query "SELECT * FROM users" -p prod
```

#### 允许写操作（危险）
```bash
# 使用 --unsafe-allow-write 绕过所有保护
xsql query "INSERT INTO logs VALUES ('test')" -p prod --unsafe-allow-write

# 或在 profile 中配置
# xsql.yaml:
# profiles:
#   prod:
#     unsafe_allow_write: true
```

**警告**：`--unsafe-allow-write` 会同时绕过客户端静态分析和服务端事务级只读，请谨慎使用。

### 绕过检测的风险提示

虽然 xsql 实现了严格的 SQL 词法分析，但仍存在理论上可绕过的边缘情况：

1. **存储过程**：如果数据库中存在存储过程，调用 `CALL sp_delete_data()` 会被拦截，但如果存储过程名称不包含危险关键字，可能误判为安全。建议在生产环境中配合数据库用户权限控制。

2. **函数副作用**：某些数据库函数可能在执行时产生副作用（如写入日志表），这在词法层面无法检测。

3. **方言差异**：不同数据库可能有特殊的语法扩展。建议仅在支持的数据库（MySQL、PostgreSQL）上使用 xsql。

**最佳实践**：
- 为 AI/自动化工具创建专用数据库用户，限制其权限
- 使用只读副本（read replica）进行查询
- 定期审计查询日志

## SSH Proxy 与 driver dial
- `database/sql` 不提供通用替换 `net.Conn` 的入口；需要依赖各 driver 的 dial hook。
- 本项目采用 **driver 自定义 dial + sshClient.Dial**（详见 `docs/ssh-proxy.md`）。
- 保留未来回退到本地端口转发的可能。

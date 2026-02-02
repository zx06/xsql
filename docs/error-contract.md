# 输出与错误契约（Error Contract）

本文件定义 xsql 对 **AI/agent** 的稳定机器接口契约。

## 1. JSON/YAML 输出顶层结构
- 成功：
  ```json
  {"ok":true,"schema_version":1,"data":{...}}
  ```
- 失败：
  ```json
  {"ok":false,"schema_version":1,"error":{"code":"...","message":"...","details":{...}}}
  ```

强约束：
- **所有机器可读输出（json/yaml）都必须保证 stderr 不混入数据。**
- `schema_version`：目前固定为 `1`（只增不改；重大变化用新版本号）。

## 2. error 对象
字段含义：
- `code`：稳定错误码（字符串，供程序判断）
- `message`：面向人的简短描述（可本地化，但 `code` 不变）
- `details`：结构化细节（可选），用于调试/自动化处理
  - **注意**：`cause` 字段不在 JSON/YAML 中暴露，仅在错误字符串输出（`Error()`）和错误解包（`Unwrap()`）中可用

安全约束：
- `details` 中不得包含明文密码、私钥、passphrase、完整 DSN/URL（可脱敏）。

## 3. 退出码（建议映射）
- 0：成功
- 2：参数/配置错误
- 3：连接错误（DB/SSH）
- 4：只读策略拦截写入
- 5：DB 执行错误
- 10：内部错误

## 4. 错误码（完整列表）

配置类：
- `XSQL_CFG_NOT_FOUND` - 配置文件未找到
- `XSQL_CFG_INVALID` - 配置无效
- `XSQL_SECRET_NOT_FOUND` - 密钥未找到（keyring）

SSH 类：
- `XSQL_SSH_AUTH_FAILED` - SSH 认证失败
- `XSQL_SSH_HOSTKEY_MISMATCH` - SSH 主机密钥不匹配
- `XSQL_SSH_DIAL_FAILED` - SSH 连接失败

DB 类：
- `XSQL_DB_DRIVER_UNSUPPORTED` - 不支持的数据库类型
- `XSQL_DB_CONNECT_FAILED` - 数据库连接失败
- `XSQL_DB_AUTH_FAILED` - 数据库认证失败
- `XSQL_DB_EXEC_FAILED` - SQL 执行失败

只读策略：
- `XSQL_RO_BLOCKED` - 写操作被只读策略拦截

内部：
- `XSQL_INTERNAL` - 内部错误

## 5. 查询结果格式

### JSON/YAML 格式
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "columns": ["id", "name", "email"],
    "rows": [
      {"id": 1, "name": "Alice", "email": "alice@example.com"},
      {"id": 2, "name": "Bob", "email": null}
    ]
  }
}
```

### Table 格式（人类可读）
```
id      name    email
----    ------  ------------------
1       Alice   alice@example.com
2       Bob     <null>

(2 rows)
```

### CSV 格式
```csv
id,name,email
1,Alice,alice@example.com
2,Bob,
```

> 注：Table 和 CSV 格式直接输出数据，不包含 `ok`、`schema_version` 等元数据。

## 6. 格式选择建议

| 场景 | 推荐格式 |
|------|----------|
| AI/程序消费 | json |
| 配置/调试 | yaml |
| 终端查看 | table (auto) |
| 数据导出 | csv |

## 7. 大结果集（计划中）
- `--jsonl`（NDJSON）：每行一个 JSON 对象，用于大结果集/流式消费

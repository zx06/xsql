# 输出与错误契约（Error Contract）

本文件定义 xsql 对 **AI/agent** 的稳定机器接口契约。

## 1. JSON/YAML 输出顶层结构
- 成功：
  ```json
  {"ok":true,"schema_version":1,...}
  ```
- 失败：
  ```json
  {"ok":false,"schema_version":1,"error":{"code":"...","message":"...","details":{...}}}
  ```

强约束：
- **所有机器可读输出（json/yaml/csv/jsonl）都必须保证 stderr 不混入数据。**
- `schema_version`：目前固定为 `1`（只增不改；重大变化用新版本号）。

## 2. error 对象
字段含义：
- `code`：稳定错误码（字符串，供程序判断）
- `message`：面向人的简短描述（可本地化，但 `code` 不变）
- `details`：结构化细节（可选），用于调试/自动化处理

安全约束：
- `details` 中不得包含明文密码、私钥、passphrase、完整 DSN/URL（可脱敏）。

## 3. 退出码（建议映射）
- 0：成功
- 2：参数/配置错误
- 3：连接错误（DB/SSH）
- 4：只读策略拦截写入
- 5：DB 执行错误
- 10：内部错误

## 4. 建议的错误码（初始集合，可扩展）
> 仅列出命名规范与常见类目；具体实现时在 `internal/errors` 维护单一来源。

- 配置类：
  - `XSQL_CFG_NOT_FOUND`
  - `XSQL_CFG_INVALID`
  - `XSQL_SECRET_NOT_FOUND`
- SSH 类：
  - `XSQL_SSH_AUTH_FAILED`
  - `XSQL_SSH_HOSTKEY_MISMATCH`
  - `XSQL_SSH_DIAL_FAILED`
- DB 类：
  - `XSQL_DB_DRIVER_UNSUPPORTED`
  - `XSQL_DB_CONNECT_FAILED`
  - `XSQL_DB_AUTH_FAILED`
  - `XSQL_DB_EXEC_FAILED`
- 只读策略：
  - `XSQL_RO_BLOCKED`

## 5. 大结果集与流式
- `--format json`：默认一次性 JSON（适合中小结果集）
- `--jsonl`：NDJSON（每行一个 JSON 对象），用于大结果集/流式消费
- `--format csv`：逐行流式

# 环境变量（ENV）约定

统一前缀：`XSQL_`。

## 1. 已实现变量
- `XSQL_PROFILE`：选择 profile
- `XSQL_FORMAT`：默认输出格式（json/yaml/table/csv）

## 2. 连接参数（计划中，当前未实现）
> 当前版本连接参数通过 config 文件的 profile 配置，ENV 支持计划在后续版本实现。

通用：
- `XSQL_DB`：`mysql` | `pg`
- `XSQL_DSN`：原生 DSN（优先级高）
- `XSQL_URL`：统一 URL（可选）

明细（当不使用 DSN/URL）：
- `XSQL_HOST`
- `XSQL_PORT`
- `XSQL_USER`
- `XSQL_PASSWORD`
- `XSQL_DATABASE`

## 3. SSH 相关（计划中，当前未实现）
- `XSQL_SSH_HOST`
- `XSQL_SSH_PORT`
- `XSQL_SSH_USER`
- `XSQL_SSH_IDENTITY_FILE`
- `XSQL_SSH_PASSPHRASE`

## 4. 约束与建议
- 优先级：CLI > ENV > Config。
- secrets 建议通过 keyring 引用或 ENV 提供；避免落盘明文。
- 复杂 profile 级别的 ENV 覆盖（如 `XSQL_PROFILE_<NAME>_HOST`）暂不支持；如要支持需通过 RFC 明确命名规则。

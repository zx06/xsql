# 环境变量（ENV）约定

统一前缀：`XSQL_`。

## 1. 基础变量
- `XSQL_PROFILE`：选择 profile
- `XSQL_FORMAT`：默认输出格式（json/yaml/table/csv）

## 2. 连接参数（推荐集合）
> 实现时可根据支持范围取舍，但命名应保持稳定。

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

## 3. SSH 相关
- `XSQL_SSH_HOST`
- `XSQL_SSH_PORT`
- `XSQL_SSH_USER`
- `XSQL_SSH_IDENTITY_FILE`
- `XSQL_SSH_PASSPHRASE`

## 4. 约束与建议
- 优先级：CLI > ENV > Config。
- secrets 建议通过 keyring 引用或 ENV 提供；避免落盘明文。
- 复杂 profile 级别的 ENV 覆盖（如 `XSQL_PROFILE_<NAME>_HOST`）第一阶段不强制支持；如要支持需通过 RFC 明确命名规则。

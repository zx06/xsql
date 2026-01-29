# 配置（Config / ENV / CLI）

## 优先级
- CLI > ENV > Config

## Profiles
建议支持多 profile：`profiles.<name>`。

## Secrets
- 默认：使用 OS keyring 保存密码/私钥 passphrase。
- config 中使用引用（示例）：
  - `password: "keyring:xsql/profiles/dev/db_password"`
- 明文作为可选：通过配置项或开关显式允许。

### Secret 解析顺序（建议）
1) 若值是 `keyring:` 引用 → 从 OS keyring 读取
2) 否则若为明文且允许明文 → 直接使用
3) 否则若为交互式 TTY → 提示输入（不回显）
4) 否则报错

## ENV 约定
见 `docs/env.md`（统一前缀 `XSQL_`，以及推荐变量集合）。

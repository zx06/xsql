# 配置（Config / ENV / CLI）

## 优先级
- CLI > ENV > Config
## Profiles

支持多 profile：`profiles.<name>`，通过 `--profile` 或 `XSQL_PROFILE` 选择。

> **默认 Profile**：如果未通过 CLI 或 ENV 指定 profile，且配置中存在名为 `default` 的 profile，则自动使用 `default` profile。

## SSH Proxies

SSH 代理可以在 `ssh_proxies` 中定义，然后在多个 profile 中复用：

```yaml
ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_ed25519
    passphrase: "keyring:ssh/passphrase"
    known_hosts_file: ~/.ssh/known_hosts

  internal-jump:
    host: jump.internal.com
    user: ops
    identity_file: ~/.ssh/ops_key
```

## 完整配置示例

```yaml
# xsql.yaml
ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_ed25519

profiles:
  # 本地 MySQL 开发环境
  dev-mysql:
    description: "本地开发 MySQL 数据库"
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    password: "keyring:dev/mysql_password"  # 使用 keyring
    database: myapp_dev
    format: table

  # 本地 PostgreSQL 开发环境（明文密码）
  dev-pg:
    db: pg
    host: 127.0.0.1
    port: 5432
    user: postgres
    password: "devpass"  # 明文密码
    database: myapp_dev
    allow_plaintext: true  # 允许明文密码

  # 通过 SSH tunnel 连接生产 MySQL
  prod-mysql:
    db: mysql
    host: db.internal.example.com  # 数据库内网地址
    port: 3306
    user: app_readonly
    password: "keyring:prod/mysql_password"
    database: myapp_prod
    ssh_proxy: bastion  # 引用预定义的 SSH 代理

  # 另一个使用同一 SSH 代理的数据库
  prod-analytics:
    db: pg
    host: analytics.internal.example.com
    port: 5432
    user: readonly
    password: "keyring:prod/analytics_password"
    database: analytics
    ssh_proxy: bastion  # 复用同一个 SSH 代理

  # 使用 DSN 直接连接
  staging:
    db: pg
    dsn: "postgres://user:pass@staging-db.example.com:5432/myapp?sslmode=require"
```

## SSH Proxy 配置项

| 字段 | 类型 | 说明 |
|------|------|------|
| `host` | string | SSH 跳板机地址 |
| `port` | int | SSH 端口（默认 22） |
| `user` | string | SSH 用户名 |
| `identity_file` | string | SSH 私钥路径 |
| `passphrase` | string | 私钥密码（支持 `keyring:` 引用） |
| `known_hosts_file` | string | known_hosts 文件路径 |
| `skip_host_key` | bool | 跳过主机密钥验证（危险） |

## Profile 配置项

| 字段 | 类型 | 说明 |
|------|------|------|
| `description` | string | 描述信息，用于区分不同数据库 |
| `db` | string | 数据库类型：`mysql` 或 `pg` |
| `dsn` | string | 原生 DSN（优先于 host/port/user 等） |
| `host` | string | 数据库主机 |
| `port` | int | 数据库端口（默认 MySQL:3306, PG:5432） |
| `user` | string | 数据库用户名 |
| `password` | string | 密码（支持 `keyring:` 引用） |
| `database` | string | 数据库名 |
| `unsafe_allow_write` | bool | 允许写操作，绕过只读保护（默认 false） |
| `allow_plaintext` | bool | 允许明文密码（默认 false） |
| `format` | string | 输出格式：json/yaml/table/csv/auto |
| `ssh_proxy` | string | SSH 代理名称（引用 `ssh_proxies` 中定义的名称） |

## Secrets

- 默认：使用 OS keyring 保存密码/私钥 passphrase。
- config 中使用引用格式：`keyring:<account>`
  - 示例：`password: "keyring:prod/db_password"`
- 明文密码需要显式允许：
  - 配置文件中设置 `allow_plaintext: true`
  - 或使用 CLI 标志 `--allow-plaintext`

### 设置 keyring 密码

keyring 引用格式为 `keyring:<account>`，其中：
- **service** 固定为 `xsql`
- **account** 是你指定的标识符（可包含 `/`）

例如 `keyring:prod/db_password` 表示 service=`xsql`，account=`prod/db_password`。

```bash
# macOS
security add-generic-password -s "xsql" -a "prod/db_password" -w "your_password"

# Linux (需要 secret-tool)
secret-tool store --label="xsql prod db" service "xsql" account "prod/db_password"

# Windows (PowerShell)
cmdkey /generic:xsql:prod/db_password /user:xsql /pass:your_password
```

### Secret 解析顺序
1. 若值是 `keyring:` 引用 → 从 OS keyring 读取（service 固定为 `xsql`）
2. 否则若为明文且允许明文 → 直接使用
3. 否则报错

## Config 文件

- 格式：YAML
- 默认查找顺序：
  1. `./xsql.yaml`（当前目录）
  2. `$HOME/.config/xsql/xsql.yaml`
- 可通过 `--config <path>` 显式指定（不存在则报错）

## ENV 约定
见 `docs/env.md`（统一前缀 `XSQL_`）。

# xsql

AI-first 的跨数据库 CLI 工具（Golang）。

## 目标
- 为 AI agent 提供稳定、可机读、可组合的数据库操作接口（CLI/未来 server/MCP）。
- 支持 MySQL / PostgreSQL，并具备可扩展的 driver 架构。
- 支持 SSH proxy（driver 自定义 dial，必要时回退本地端口转发）。

## 状态
- 已实现核心功能：
  - `xsql query` - 执行只读 SQL 查询（支持 MySQL / PostgreSQL）
  - `xsql spec` - 导出 tool spec（供 AI/agent 自动发现）
  - `xsql version` - 版本信息
- 支持 SSH tunnel（通过 driver dial hook）
- 支持 keyring 密钥管理
- 支持 YAML 配置文件 + profile

## 快速开始

```bash
# 安装
go install github.com/zx06/xsql/cmd/xsql@latest

# 创建配置文件 xsql.yaml
cat > xsql.yaml << 'EOF'
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    password: "keyring:xsql/dev/password"  # 或明文（需 --allow-plaintext）
    database: test
    read_only: true
EOF

# 执行查询
xsql query "SELECT 1" --profile dev --format json
```

### 通过 SSH tunnel 连接

```yaml
profiles:
  prod:
    db: pg
    host: db.internal
    port: 5432
    user: app
    password: "keyring:xsql/prod/password"
    database: mydb
    read_only: true
    ssh_host: jump.example.com
    ssh_user: admin
    ssh_identity_file: ~/.ssh/id_ed25519
```

## 输出格式

所有机器输出遵循统一契约：

```json
// 成功
{"ok":true,"schema_version":1,"data":{...}}

// 失败
{"ok":false,"schema_version":1,"error":{"code":"XSQL_...","message":"...","details":{...}}}
```

支持格式：`--format json|yaml|table|csv|auto`（非 TTY 默认 json）

## 文档索引
- 设计总览：`docs/architecture.md`
- CLI 约定与输出/错误规范：`docs/cli-spec.md`
- 输出与错误契约：`docs/error-contract.md`
- 配置与 Profile/Secret：`docs/config.md`
- 环境变量约定：`docs/env.md`
- SSH Proxy：`docs/ssh-proxy.md`
- 数据库驱动与只读策略：`docs/db.md`
- 开发指南：`docs/dev.md`
- AI-first 约定：`docs/ai.md`
- 设计变更（RFC）：`docs/rfcs/README.md`

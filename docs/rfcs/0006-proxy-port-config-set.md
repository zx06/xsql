# RFC 0006: Proxy Port Config & Config Set Command

Status: Accepted

## 摘要
增加两项能力：（1）支持在 profile 配置中指定 proxy 本地端口，端口冲突时交互式询问用户；（2）新增 `xsql config set` 和 `xsql config init` 命令，降低配置复杂度。两项变更涉及 config schema 新增字段、新增 CLI 命令、新增错误码。

## 背景 / 动机
- 当前痛点：
  - `xsql proxy` 每次都需通过 `--local-port` 指定端口，无法在配置文件中固定。
  - 配置文件需要手动编辑 YAML，对新用户不友好。
- 目标：
  - Profile 可配置 `local_port`，proxy 命令自动使用；端口被占用时交互提示。
  - 提供 `config set` 快速修改配置、`config init` 生成模板。
- 非目标：
  - 不改变现有 proxy 的 SSH 连接逻辑。
  - 不实现 `config get` 或 `config delete`（可后续扩展）。

## 方案（Proposed）

### 用户视角（CLI/配置/输出）

#### 1. Profile `local_port` 字段
```yaml
profiles:
  prod-mysql:
    db: mysql
    host: db.internal.example.com
    port: 3306
    local_port: 13306  # 新增：proxy 本地端口
    ssh_proxy: bastion
```

端口优先级：`--local-port` flag > `profile.local_port` > 0（自动分配）

端口冲突处理：
- 仅当端口来源于**配置文件**（非 CLI flag）时，才提示用户选择。
- TTY 环境：询问 "Port 13306 is in use. [R]andom port / [Q]uit?"
- 非 TTY 环境：返回错误 `XSQL_PORT_IN_USE`，退出码 10。

#### 2. `xsql config init`
```bash
xsql config init                    # 创建 ~/.config/xsql/xsql.yaml
xsql config init --path ./xsql.yaml # 指定路径
```

#### 3. `xsql config set`
```bash
xsql config set profile.dev.host localhost
xsql config set profile.dev.port 3306
xsql config set profile.dev.db mysql
xsql config set ssh_proxy.bastion.host bastion.example.com
xsql config set ssh_proxy.bastion.user admin
```

输出（JSON）：
```json
{"ok":true,"schema_version":1,"data":{"config_path":"/path/to/xsql.yaml","key":"profile.dev.host","value":"localhost"}}
```

### 新增错误码
| Code | 含义 | 退出码 |
|------|------|--------|
| `XSQL_PORT_IN_USE` | 端口被占用 | 10 |

### 技术设计（Architecture）

#### 涉及模块
- `internal/config/types.go`：Profile 新增 `LocalPort` 字段
- `internal/config/write.go`：新增配置写入能力
- `internal/proxy/proxy.go`：端口冲突检测
- `internal/errors/codes.go`：新增错误码
- `cmd/xsql/proxy.go`：读取 config local_port、端口冲突交互
- `cmd/xsql/config.go`：新增 config 命令组

#### 兼容性策略
- `local_port` 是新增字段，默认为 0（不影响现有行为）
- config 命令是全新命令，不影响现有命令
- 只增不改

## 备选方案（Alternatives）
- 方案 A（采用）：profile 中直接加 `local_port` 字段
- 方案 B：在 proxy 子节点增加配置 —— 过度设计，当前场景不需要

## 兼容性与迁移（Compatibility & Migration）
- 不破坏兼容：所有新增字段有零值默认
- 无需迁移

## 安全与隐私（Security/Privacy）
- config set 的 password 字段建议使用 `keyring:` 引用，不阻止明文但遵循已有 allow_plaintext 机制
- config init 模板中不包含真实密码

## 测试计划（Test Plan）
- 单元测试：
  - config write/update 逻辑
  - proxy port conflict detection
  - config set key parsing
- E2E 测试：
  - `xsql config init` 创建文件
  - `xsql config set` 修改配置
  - proxy 使用 config local_port
  - proxy 端口冲突场景

## 未决问题（Open Questions）
- 无

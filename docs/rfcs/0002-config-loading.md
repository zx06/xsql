# RFC 0002: Config 文件加载与 Profile 选择

Status: Implemented

## 摘要
为 xsql 增加第一阶段的配置文件能力：默认从固定路径查找 YAML 配置，并支持通过 `--config` 指定路径；支持 `profiles.<name>` 多 profile，并通过 `--profile` / `XSQL_PROFILE` 选择；支持 `ssh_proxies.<name>` 定义可复用的 SSH 代理配置；所有配置项遵循 **CLI > ENV > Config** 优先级。

## 背景 / 动机
- 需要稳定可预期的参数来源与优先级（AI-first）。
- 为后续 DB/SSH/secret 能力提供一致的配置入口。
- 多个数据库可能共享同一个 SSH 代理，需要支持复用以简化配置。

## 方案（Proposed）
### 用户视角（CLI/配置/输出）
#### 配置文件
- 格式：YAML
- 默认查找路径（按顺序，取第一个存在的）：
  1) `./xsql.yaml`
  2) `$HOME/.config/xsql/xsql.yaml`
- 显式指定：`--config <path>`（若不存在则报错）

#### SSH Proxies
- 配置文件支持：`ssh_proxies.<name>` 定义可复用的 SSH 代理
- Profile 通过 `ssh_proxy: <name>` 引用预定义的代理
- 多个 profile 可共享同一个 SSH 代理配置

```yaml
ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_rsa

profiles:
  prod-db1:
    db: mysql
    host: db1.internal
    ssh_proxy: bastion  # 引用代理

  prod-db2:
    db: pg
    host: db2.internal
    ssh_proxy: bastion  # 复用同一代理
```

#### Profile
- 配置文件支持：`profiles.<name>`
- 选择优先级：`--profile` > `XSQL_PROFILE` >（若存在）`profiles.default` > 空（不使用 profile）

#### 优先级
- 所有可配置项：**CLI > ENV > Config**（详见 `docs/config.md` / `docs/env.md`）

#### 错误码/退出码
- 配置文件不存在：`XSQL_CFG_NOT_FOUND`（exit=2）
- 配置文件格式/字段非法：`XSQL_CFG_INVALID`（exit=2）
- SSH 代理引用不存在：`XSQL_CFG_INVALID`（exit=2）

### 技术设计（Architecture）
- 新增 `internal/config`：
  - 负责查找/加载 YAML
  - 负责 profile 选择
  - 负责 ssh_proxy 引用解析
  - 负责合并（CLI > ENV > Config）
- `cmd/xsql` 仅绑定 flags 并调用 `internal/config`，不得把 cobra 类型泄漏到 internal。

## 兼容性与迁移
- 新增能力；未提供配置文件/ENV/flag 时行为不变。

## 安全与隐私
- 解析配置时不得将 secrets 原文写入日志/错误 details。

## 测试计划
- 单测：
  - 显式 config path 缺失/非法
  - profile 选择优先级
  - format 等字段的 CLI/ENV/Config 合并优先级
  - ssh_proxy 引用解析与验证
  - ssh_proxy 引用不存在时报错

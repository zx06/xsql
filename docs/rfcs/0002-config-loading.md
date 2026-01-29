# RFC 0002: Config 文件加载与 Profile 选择

Status: Implemented

## 摘要
为 xsql 增加第一阶段的配置文件能力：默认从固定路径查找 YAML 配置，并支持通过 `--config` 指定路径；支持 `profiles.<name>` 多 profile，并通过 `--profile` / `XSQL_PROFILE` 选择；所有配置项遵循 **CLI > ENV > Config** 优先级。

## 背景 / 动机
- 需要稳定可预期的参数来源与优先级（AI-first）。
- 为后续 DB/SSH/secret 能力提供一致的配置入口。

## 方案（Proposed）
### 用户视角（CLI/配置/输出）
#### 配置文件
- 格式：YAML
- 默认查找路径（按顺序，取第一个存在的）：
  1) `./xsql.yaml`
  2) `$HOME/.config/xsql/xsql.yaml`
- 显式指定：`--config <path>`（若不存在则报错）

#### Profile
- 配置文件支持：`profiles.<name>`
- 选择优先级：`--profile` > `XSQL_PROFILE` >（若存在）`profiles.default` > 空（不使用 profile）

#### 优先级
- 所有可配置项：**CLI > ENV > Config**（详见 `docs/config.md` / `docs/env.md`）

#### 错误码/退出码
- 配置文件不存在：`XSQL_CFG_NOT_FOUND`（exit=2）
- 配置文件格式/字段非法：`XSQL_CFG_INVALID`（exit=2）

### 技术设计（Architecture）
- 新增 `internal/config`：
  - 负责查找/加载 YAML
  - 负责 profile 选择
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

# AGENTS.md

本文件用于指导 AI agent / 自动化代码生成工具在 **xsql** 项目中进行一致、可维护、可扩展的开发。

## 1. 项目定位（必须理解）
- **xsql**：AI-first 的跨数据库 CLI 工具（Golang）。
- 核心目标：
  - 对 AI/agent 友好：稳定的机器可读输出、稳定错误码/退出码、可导出 tool spec。
  - 支持 MySQL/PostgreSQL，且 driver 架构可扩展。
  - 支持 **Go 内实现** SSH proxy（driver 自定义 dial，必要时回退本地端口转发）。

> 任何改动都不得破坏既定的输出契约与错误码稳定性。

## 2. 文档是事实来源（Source of Truth）
实现前先阅读：
- `docs/architecture.md`
- `docs/cli-spec.md`
- `docs/error-contract.md`
- `docs/config.md`
- `docs/env.md`
- `docs/db.md`
- `docs/ssh-proxy.md`
- `docs/ai.md`

若实现与文档冲突：优先更新文档并说明原因，再实现。

**强约束（Docs/RFC-first）**：
- **以文档为准**：实现必须与 `docs/*` 一致。
- **方案发生变化必须先改文档**：任何会改变对外行为/契约/架构的变更，必须先通过文档（建议用 RFC）明确。
- **实现不得领先于文档**：合并时必须保证"文档与实现一致"。
- RFC 维护位置：`docs/rfcs/`（见 `docs/rfcs/README.md`）。

## 3. 架构与目录约束（不要越界）
推荐结构（实现时请遵循）：
- `cmd/xsql`：CLI 入口（只做参数解析、调用 app、输出）
- `internal/app`：应用编排（核心入口，供未来 MCP/TUI/Web 复用）
- `internal/config`：配置加载/合并/校验 + profile
- `internal/secret`：keyring/加密/明文兼容
- `internal/db`：driver registry + 执行引擎
- `internal/ssh`：SSH proxy（driver dial/端口转发）+ ssh_config（可选）
- `internal/output`：json/yaml/table/csv + 流式写
- `internal/errors`：错误码/退出码/结构化错误
- `internal/spec`：tool spec 导出

强制规则：
- **核心逻辑不得依赖 CLI 框架类型**（cobra 的 Command 等不应出现在 internal）。
- 输出/日志严格分流：**stdout=数据，stderr=日志**。

## 4. AI-first 输出与错误契约（强约束）
### 4.1 输出
- 成功：`{"ok":true,...}`
- 失败：`{"ok":false,"error":{"code":"...","message":"...","details":{...}}}`
- 非 TTY 默认 JSON；TTY 默认 table（详见 `docs/cli-spec.md`）。

### 4.2 错误码与退出码
- 必须集中在 `internal/errors` 维护（单一来源）。
- 退出码建议：0/2/3/4/5/10（详见 `docs/cli-spec.md`）。
- 新增错误码：
  1) 先在文档与代码的错误码表登记
  2) 保持向后兼容（只增不改、不复用旧含义）

## 5. 配置/ENV/CLI 合并规则（强约束）
- 优先级：**CLI > ENV > Config**。
- ENV 前缀：`XSQL_`。
- Secrets：默认使用 OS keyring；config 明文仅作为可选能力（详见 `docs/config.md`）。
- 不允许在日志/错误 details 中输出明文密码/私钥内容。

## 6. 数据库与只读策略（强约束）
- 目标数据库：MySQL + PostgreSQL。
- 可扩展性：通过 driver registry 扩展新 DB。
- **默认只读**：防止误操作，默认启用双重只读保护（SQL 静态分析 + 数据库事务级只读）。
- **可启用写操作**：使用 `--unsafe-allow-write` 或配置 `unsafe_allow_write: true`。
- 只读拦截必须返回明确错误码（退出码=4）。

## 7. SSH Proxy（必须支持）
- 默认方案：**driver 自定义 dial + sshClient.Dial**（不打开本地监听端口）。
- 回退方案：本地端口转发（类似 `ssh -L`），用于不支持 dial hook 的 driver/特殊场景。
- 默认启用 known_hosts 校验；如提供跳过开关必须明确标注风险。
- `ssh_config` 解析是可选增强：能做则做，做不了不阻塞主功能。

## 8. 依赖与工程实践
- Go 版本以 `go.mod` 为准。
- 依赖选择优先标准库；第三方库需：
  - 成熟、维护活跃、许可证兼容
  - 不引入巨大依赖树
- 任何新增依赖都要说明用途与替代方案。

## 9. 测试与验证（提交前必须）
最低要求：
- 单元测试覆盖：
  - 配置优先级合并
  - 连接参数解析（dsn/url）
  - 只读 SQL 判定（允许/拒绝用例）
  - 输出序列化（json/yaml/csv）
- 确保日志不会污染 stdout。

## 10. 安全与隐私（红线）
- 不把任何 secret 写入仓库（包括示例配置中的真实值）。
- 默认安全：known_hosts 校验开启、只读模式保守拦截。
- 错误 details 中避免包含：密码、私钥、完整连接串（可脱敏）。

## 11. 项目开发流程（AI agent 最佳实践，强烈建议遵循）
> 目标：让每次变更都可复现、可验证、可回滚，并保持对 AI/agent 的接口稳定。

### 11.1 接需求/任务拆解
1) 复述目标与验收标准（输出/错误码/行为边界）
2) 明确影响范围：CLI / config / db / ssh / output / spec
3) 如果存在歧义，先在 issue/讨论里确认（避免“猜需求”）

### 11.2 设计先行（Docs-first / RFC-first）
1) 先阅读/定位相关文档（第 2 节）
2) 若要新增/修改契约或方案（命令、flag、输出字段、错误码、配置 schema、ssh 行为、driver 抽象等）：
   - **先写/更新 RFC**：`docs/rfcs/NNNN-*.md`（使用 `docs/rfcs/0000-template.md`）
   - **再同步更新对应 `docs/*`**（尤其 `docs/cli-spec.md`、`docs/ai.md`）
   - 最后再实现代码

### 11.3 实现顺序（Contract-first）
按以下顺序实现，降低返工：
1) `internal/errors`：错误码/退出码/错误结构
2) `internal/output`：序列化与 stdout/stderr 分流
3) `internal/config`：参数合并（CLI > ENV > Config）与 profile
4) `internal/db`：driver registry + 最小 query/exec
5) `internal/ssh`：SSH dial 集成（driver dial）
6) `cmd/xsql`：命令与 flag 绑定（薄层）

### 11.4 变更粒度（Small PR）
- 每次变更只做一件事：
  - 一个命令、一个输出格式、一个 driver、或一个 ssh 能力
- 避免跨模块“大重构”；必须重构时先独立提交 refactor，再提交功能。

### 11.5 测试与验证（必须）
1) 先跑：`go test ./...`
2) 新增能力必须附带对应单元测试（见第 9 节最低覆盖面）
3) 验证 stdout/stderr：机器输出不可被日志污染

### 11.6 交付（Definition of Done）
- 文档与实现一致
- 错误码/退出码稳定且有覆盖
- `go test ./...` 通过
- 没有把 secret 写入仓库；日志/错误不泄露敏感信息

## 12. AI agent 工作方式（执行准则）
1) 先读相关 docs，明确契约与边界
2) 用最小改动实现目标（尽量少文件、少行数）
3) 先保证 JSON 输出与错误码稳定
4) 添加/更新对应文档（仅与本改动直接相关）
5) 运行已有测试/构建命令验证

## 13. 变更准则（最重要）
- 不要“顺手重构”。
- 不要改变已有输出字段含义。
- 新功能必须可关闭/可回退（尤其是安全相关选项）。

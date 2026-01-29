# xsql

AI-first 的跨数据库 CLI 工具（Golang）。

## 目标
- 为 AI agent 提供稳定、可机读、可组合的数据库操作接口（CLI/未来 server/MCP）。
- 支持 MySQL / PostgreSQL，并具备可扩展的 driver 架构。
- 支持 SSH proxy（driver 自定义 dial，必要时回退本地端口转发）。

## 状态
- 当前处于脚手架阶段：已建立文档/规范（AI-first 契约、RFC 流程等），代码实现尚未开始。

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

## 规划
详细实施计划见 Copilot session 的 `plan.md`（不提交到仓库）。

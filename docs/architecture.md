# 架构设计（Architecture）

## 分层原则
- 核心能力（config/db/ssh/output）与 CLI 解耦，便于未来扩展 MCP/TUI/Web。
- 输出与错误遵循稳定契约，优先保证 AI/agent 可消费。

## 建议目录结构
```
/cmd/xsql              # CLI 入口
/internal/app          # 应用编排（解析参数->执行->返回结构化结果）
/internal/config       # 配置加载/合并/校验 + profiles
/internal/secret       # keyring/加密/明文兼容
/internal/db           # driver registry + 执行引擎
/internal/db/mysql     # MySQL 驱动实现
/internal/db/pg        # PostgreSQL 驱动实现
/internal/mcp          # MCP Server 实现
/internal/ssh          # SSH proxy（driver dial，必要时回退端口转发）+ ssh_config（可选）
/internal/proxy        # 端口转发代理（ssh -L 语义）
/internal/output       # json/yaml/table/csv + 流式输出
/internal/spec         # tool spec 导出（JSON schema）
/internal/errors       # 错误码/退出码/可机读错误
/internal/log          # 日志（stderr）
```

## 关键扩展点
- DB driver：通过 registry 注册，实现新数据库无需改动核心逻辑。
- 输出 formatter：通过接口注册新格式。
- Frontend：CLI/MCP/TUI/Web 仅是调用 `internal/app` 的不同适配层。

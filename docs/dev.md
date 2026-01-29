# 开发指南（Dev Guide）

## 基本原则
- 机器输出（json/yaml/csv）只写 stdout；日志/调试信息只写 stderr。
- 核心逻辑放 `internal/*`，CLI 仅做参数解析与输出。
- 为命令/输出/错误码维护稳定契约（AI-first）。

## 推荐依赖（待实现时引入）
- CLI：cobra
- Config：viper 或自研（需支持优先级合并）
- DB：database/sql + mysql driver + pgx stdlib
- Secrets：keyring
- SSH：x/crypto/ssh
- Output：json/yaml/csv + tablewriter（或等价）

## 快速开始（占位）
待实现 CLI 后补充。

# CLI 规范（AI-first）

## 设计目标
- 机器可读：默认在非 TTY 输出 JSON；错误也结构化。
- 稳定：字段名、错误码、退出码保持兼容。

## 输出约定
- 契约总览见：`docs/error-contract.md`
- 成功：`{"ok":true,"schema_version":1,...}`
- 失败：`{"ok":false,"schema_version":1,"error":{"code":"...","message":"...","details":{...}}}`

## 退出码建议
- 0：成功
- 2：参数/配置错误
- 3：连接错误
- 4：只读策略拦截写入
- 5：DB 执行错误
- 10：内部错误

## 格式与大结果集
- `--format json|yaml|table|csv`
- 建议增加 `--jsonl`（NDJSON）用于大结果集流式输出。
- table 默认 `--pager=auto`（仅 TTY 且行数超过阈值启用）。

## 参数来源优先级
- CLI > ENV > Config

## 全局 flags（第一阶段）
- `--config <path>`：显式指定 YAML 配置文件路径
- `--profile <name>`：选择 profile（等价 ENV：`XSQL_PROFILE`）

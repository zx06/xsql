# AI-first 约定

## 目标
让 LLM/agent 可以可靠调用：
- 输出可预测、可机读
- 错误码稳定
- 命令与参数可被自动发现（tool spec）

## 规范建议
- 非 TTY 默认输出 JSON；TTY 默认 table。
- 错误对象：`code/message/details`；并保证退出码与 code 对应。
- 提供 `xsql spec --format json` 导出：
  - commands/flags/env mapping
  - output schema
  - error codes

## 兼容性
- 对 JSON 输出字段做版本化（`schema_version`），新字段只增不改；详细契约见 `docs/error-contract.md`。

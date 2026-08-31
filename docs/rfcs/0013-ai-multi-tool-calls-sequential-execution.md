# RFC 0013: AI Multi Tool Call Sequential Execution and Schema Dialect Guidance

Status: Implemented

## 摘要
本 RFC 优化 `xsql ai` 的 ReAct Agent 调用机制：支持模型单轮返回多个 Tool Call 时的**按序依次调度与执行**（Sequential Execution），彻底消除此前由于模型返回多 Tool Call 导致的硬性中断崩溃；同时升级 System Prompt 模板，显式注入当前数据库名与精确的数据库方言元数据查询指引（MySQL vs PostgreSQL），杜绝 Agent 跨方言盲目试探与反复死循环。

## 背景 / 动机
1. **多 Tool Call 抛错崩溃**：部分兼容 OpenAI 协议的模型（如 DeepSeek、Qwen、第三方网关）在特定场景下会忽略 `parallel_tool_calls: false`，单次返回多个工具调用（例如连续查询多个元数据表或一次性生成查询加分析代码）。此前 `internal/ai/client.go` 在 `len(ToolCalls) > 1` 时直接返回硬错误并阻断整个会话。
2. **数据库方言与 Schema 盲目探测**：在 MySQL 数据库环境下，AI 偶发会使用 PostgreSQL 的 `table_schema = 'public'` 发起查询导致查空，随后反复调用 JS 进行空转分析，浪费交互步数与 Token。

## 架构与核心设计

### 1. ToolAction 与 AIResponse 数据结构重构 (`internal/ai`)
定义清晰的 `ToolAction` 列表切片：
```go
type ToolAction struct {
    ID          string       `json:"id,omitempty"`
    Type        ResponseType `json:"type"`
    SQL         string       `json:"sql,omitempty"`
    JSCode      string       `json:"js_code,omitempty"`
    DatasetID   string       `json:"dataset_id,omitempty"`
    Format      string       `json:"format,omitempty"`
    FilePath    string       `json:"filepath,omitempty"`
    Explanation string       `json:"explanation,omitempty"`
}

type AIResponse struct {
    Type        ResponseType `json:"type"`
    SQL         string       `json:"sql,omitempty"`
    JSCode      string       `json:"js_code,omitempty"`
    DatasetID   string       `json:"dataset_id,omitempty"`
    Format      string       `json:"format,omitempty"`
    FilePath    string       `json:"filepath,omitempty"`
    Explanation string       `json:"explanation,omitempty"`
    Actions     []ToolAction `json:"actions,omitempty"`
}
```
当模型返回 1 个或多个 Tool Call 时，`client.go` 将其完整反序列化并填充至 `Actions` 切片中，不再抛错。

### 2. TUI 多 Tool Call 顺序调度队列 (`internal/tui`)
- TUI 维护 `pendingActions []ai.ToolAction` 队列。
- 当收到包含多个 Actions 的响应时，将动作入队并依次触发执行。
- 无论执行 SQL、JS 还是文件导出，当前 Action 完成后检查队列：
  - 若有剩余 Action，自动弹出并执行下一个 Action，执行结果持续记入 `chatHistory`。
  - 若当前批次所有 Action 全部执行完毕，调用 `runAgentStepCmd()` 将完整的上下文结果发送给 AI 进行下一轮总结或推理。

### 3. System Prompt 数据库上下文与调用规范 (`internal/ai/prompt.go`)
- 注入 `TARGET DATABASE: %s (Dialect: %s)` 显式标识当前连接数据库与方言。
- 规范 Agent 调用行为：在执行 SQL、运行 JS 分析或导出文件时统一使用标准的结构化 Tool Calling 协议。

---

## 修订记录 (Revision History)

### 2026-08: 报告导出工具拆分、严格 Schema 校验自动重试与路径展开
1. **文件导出职责分离**：
   - `export_data` 专注于原始数据集转储，仅支持 `csv` 和 `json` 格式，不再支持 `markdown`。
   - 新增 `export_report(content, filepath, explanation)` 工具，专用于将 LLM 总结生成的结构化富文本 Markdown 报告写入本地文件。
2. **路径自动展开 (Tilde Expansion)**：
   - 导出路径支持 `~/` 波浪号展开为用户家目录（`$HOME`），防止误拼接为相对路径。
3. **严格 Schema 校验与自愈重试**：
   - 保持严格 JSON 反序列化校验。
   - 当模型返回非法/畸形 Tool Call 参数时，由 TUI Agent 循环捕获并向上下文注入错误反馈，自动触发重试（最大 2 次），无需人工干预输入“继续”。


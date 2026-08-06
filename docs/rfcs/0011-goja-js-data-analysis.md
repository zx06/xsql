# RFC 0011: Integration of goja JS Engine and Session DataStore for AI Data Analytics

Status: Proposed

## 摘要
本 RFC 提出在 `xsql` / `xsql-ai` 中集成纯 Go 实现的 `goja` JavaScript 虚拟机（100% Zero CGO），并构建 **Session DataStore（会话数据集存储与召回）** 机制。

## 背景 / 动机
- 当前 `xsql-ai` 仅支持 SQL 交互与表格展示，缺少数据二次聚合计算、跨查询结果 Join/比对以及结构化导出文件（CSV/JSON/Markdown）的能力。
- 将海量原始数据全量透传给大模型（LLM）会导致上下文爆炸（Context Overflow）与高昂 Token 成本，且存在数据隐私泄露红线。

## 架构与核心设计

### 1. 零 CGO JS 引擎 (`internal/js`)
- 使用 `github.com/dop251/goja` 在纯 Go 内存沙箱中执行 AI 动态生成的 JS 数据分析代码。
- 支持 Context 超时打断（默认 1 分钟，可配置 `js_timeout`）。

### 2. Session 数据集存储与召回 (`internal/session`)
- 本地维护 `SessionDataStore`，为每次 SQL 执行成功的 QueryResult 分配唯一 ID（`res1`, `res2`, ...）。
- 向大模型上下文仅提供轻量 **Dataset Catalog** 元数据目录，LLM 可以在后续多轮对话中指定 `res1`, `res2` 召回历史数据并在 JS 中做跨数据集 Join 或计算。

### 3. 外层文件导出 (`internal/export`)
- JS 仅负责数据计算与转换；由外层 Go 宿主层统一执行安全的磁盘文件写入（CSV / JSON / Markdown）。

### 4. AI Tool Calling (`internal/ai`)
- 新增 Tool：`execute_javascript(js_code: string, explanation: string)`。
- AI 可先通过 `execute_sql` 查出数据，再调用 `execute_javascript` 完成分析与导出。

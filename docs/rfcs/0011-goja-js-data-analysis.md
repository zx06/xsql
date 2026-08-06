# RFC 0011: Integration of goja JS Engine, Session DataStore, and ReAct Tool Agent Loop

Status: Proposed

## 摘要
本 RFC 提出在 `xsql` / `xsql-ai` 中集成纯 Go 实现的 `goja` JavaScript 虚拟机（100% Zero CGO），构建 **Session DataStore（会话数据集存储与召回）** 机制，并实现无硬编码的 **ReAct Tool Agent Loop 循环推理**。

## 背景 / 动机
- 当前 `xsql-ai` 仅支持 SQL 交互与表格展示，缺少数据二次聚合计算、跨查询结果 Join/比对以及结构化导出文件（CSV/JSON/Markdown）的能力。
- 将海量原始数据全量透传给大模型（LLM）会导致上下文爆炸（Context Overflow）与高昂 Token 成本，且存在数据隐私泄露红线。

## 架构与核心设计

### 1. 零 CGO JS 引擎 (`internal/js`)
- 使用 `github.com/dop251/goja` 在纯 Go 内存沙箱中执行 AI 动态生成的 JS 数据分析代码。
- 规定 JS 代码必须遵循 ES5 (ECMAScript 5.1) 标准语法。
- 支持 Context 超时打断（默认 1 分钟，可配置 `js_timeout`），并自动捕获 `console.log` 输出。
- 包含 AI 自动重试修正机制（上限 3 次）。

### 2. Session 数据集存储与召回 (`internal/session`)
- 本地维护 `SessionDataStore`，为每次 SQL 执行成功的 QueryResult 分配唯一 ID（`res1`, `res2`, ...）。
- 向大模型上下文仅提供轻量 **Dataset Catalog** 元数据目录，LLM 可以在后续多轮对话中指定 `res1`, `res2` 召回历史数据并在 JS 中做跨数据集 Join 或计算。

### 3. 外层文件导出 (`internal/export`)
- JS 仅负责数据计算与转换；由外层 Go 宿主层统一执行安全的磁盘文件写入（CSV / JSON / Markdown），并强制人机交互二次确认（Human-in-the-loop）。

### 4. ReAct Tool Agent Loop 架构 (`internal/ai` & `internal/tui`)
AI 具备 4 大解耦工具：
1. `execute_sql`: 数据库 SQL 查询
2. `execute_javascript`: ES5 沙箱数据二次清洗与聚合
3. `render_table`: 交互式 TUI 表格组件渲染
4. `export_data`: 文件导出（含人机交互确认卡片）

所有 Tool Calls 默认在 TUI 容器中折叠内嵌呈现（`Ctrl+O` 展开/折叠，`Ctrl+P`/`Ctrl+N` 切换焦点），且交互末尾必定以 LLM 自然语言 Markdown 分析报告总结收尾。

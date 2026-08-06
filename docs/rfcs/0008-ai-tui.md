# RFC 0008: AI TUI Interactive Mode and Standalone `xsql-ai` Binary

Status: Accepted

## 摘要
本 RFC 提出为 `xsql` 增加交互式 AI 模式（TUI）及独立终端可执行程序 `cmd/xsql-ai`。用户可以通过自然语言在终端提问，AI 自动结合当前数据库的 Schema 结构生成相应的 SQL 语句，并在 TUI 界面中提供可视化预览、编辑与一键安全执行。

## 背景 / 动机
- **当前痛点**：用户在面对复杂的数据库表结构时，手写 SQL 查询门槛较高，传统 CLI 查询缺少交互式的自然语言转 SQL 辅助。
- **目标**：
  1. 提供独立终端程序 `xsql-ai` 及主程序子命令 `xsql ai`。
  2. 支持标准 OpenAI 兼容接口配置（兼容 OpenAI, DeepSeek, Ollama 等）。
  3. 自动抽取 Profile 对应的数据库 Schema，作为上下文送给 LLM 生成对应 DB 语法的 SQL。
  4. 采用 Charm (Bubbletea / Lipgloss) 编写现代、美观、响应式的 TUI 界面，支持 SQL 预览、编辑与快捷执行。
  5. 严格保留 `xsql` 既有的只读策略保护（双重只读拦截：SQL 静态检测 + 事务级 READ ONLY）。
- **非目标**：
  - 本 RFC 不试图在 CLI 内实现大模型训练或复杂的全表数据检索，仅聚焦于辅助 SQL 编写与交互式查询执行。

## 方案（Proposed）

### 用户视角（CLI/配置/输出）
1. **独立程序与 CLI 命令**：
   - 独立可执行程序：`xsql-ai -p <profile> [--prompt "query"]`
   - 主程序子命令：`xsql ai -p <profile>`
2. **配置文件扩展**：
   在 `xsql.yaml` 中新增 `ai` 配置块：
   ```yaml
   ai:
     provider: openai
     base_url: https://api.openai.com/v1
     api_key: keyring:ai_key
     model: gpt-4o
     max_tokens: 2048
   ```
3. **优先级与凭据**：
   - 合并优先级：`CLI flags > ENV (XSQL_AI_API_KEY, XSQL_AI_BASE_URL, XSQL_AI_MODEL) > Config`。
   - API Key 支持 `keyring:` 引用。

### 技术设计（Architecture）
- **涉及模块**：
  - `cmd/xsql-ai/main.go`：独立 CLI 程序入口。
  - `cmd/xsql/ai.go`：`xsql ai` 适配入口。
  - `internal/config`：配置类型扩展与解析。
  - `internal/ai`：OpenAI HTTP 客户端、Prompt 组装服务。
  - `internal/tui`：Bubbletea UI 架构（Header、Viewport、SQL Preview Card、Textarea）。
  - `internal/app`：复用 `DumpSchema` 与 `Query`。

### 安全与隐私（Security/Privacy）
- 默认严格只读策略，除非显式设置 `--unsafe-allow-write`，否则不允许写 SQL 执行。
- 不会把明文 API Key 输出在日志或错细节中。

### 测试计划（Test Plan）
- **单元测试**：配置合并解析、Prompt 组装、OpenAI 响应解析、TUI 状态机逻辑测试。
- **E2E 测试**：在 `tests/e2e/ai_test.go` 中启动 Mock HTTP API Server 并通过 IO Pipe 输入按键模拟全闭环交互。

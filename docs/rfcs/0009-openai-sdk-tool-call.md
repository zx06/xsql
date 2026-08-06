# RFC 0009: Migrate OpenAI Integration to Official SDK and Tool Call

Status: Proposed

## 摘要
本 RFC 提出将 `xsql` 项目中的 AI 客户端重构为使用 OpenAI 官方 Go SDK (`github.com/openai/openai-go`)，并将 SQL 生成与解析机制从手动字符串/JSON 解析改为标准 OpenAI Tool Call (`execute_sql`) 模式。

## 背景 / 动机
- **当前问题**：
  1. `internal/ai/client.go` 自行实现了 HTTP 请求封装，维护成本高且不易支持高级特性。
  2. `internal/ai/service.go` 通过 System Prompt 强求模型输出 JSON 字符串，并通过正则表达式/`json.Unmarshal` 提取 `sql` 和 `explanation`，解析脆弱且容易由于 markdown 格式干扰出错。
- **目标**：
  1. 引入 `github.com/openai/openai-go` 官方 SDK。
  2. 定义 `execute_sql(sql string, explanation string)` 函数工具，由模型通过 Tool Call 返回结构化参数。
  3. 保留对无 Tool Call 场景（文本回复）的兼容容错。

## 方案（Proposed）

### 技术设计
1. **官方 SDK 集成**：
   - 依赖：`github.com/openai/openai-go`
   - 初始化：使用 `openai.NewClient(option.WithAPIKey(...), option.WithBaseURL(...), option.WithHTTPClient(...))`。
2. **Tool Calling 定义**：
   - 工具名称：`execute_sql`
   - 工具描述：Execute or present generated SQL query based on database schema and user intent.
   - 参数 Schema：
     - `sql`: (string) 针对特定 DB 语法的 SQL 查询语句。
     - `explanation`: (string) 对查询意图或无法生成 SQL 的说明解释。
3. **响应处理逻辑**：
   - 若模型响应包含 `execute_sql` 的 Tool Call，则解析 JSON 参数提取 `sql` 与 `explanation`。
   - 若模型仅返回文本（无 Tool Call），则设置 `sql=""`，并将文本存入 `explanation`。

### 测试计划
- 单元测试：`internal/ai/service_test.go` 升级 Mock API 响应为 Tool Call 消息体格式。
- E2E 测试：`tests/e2e/ai_test.go` 升级 Mock API 响应。

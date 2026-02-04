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

## MCP Server
xsql 提供了 MCP (Model Context Protocol) Server 模式，允许 AI 助手通过标准 MCP 协议访问数据库查询能力。

### 启动方式
```bash
xsql mcp server
```

### Streamable HTTP 传输
需要通过 `streamable_http` 启动，并强制要求鉴权：
```bash
xsql mcp server --transport streamable_http --http-addr 127.0.0.1:8787 --http-auth-token "your-token"
```

### MCP Tools
MCP Server 提供以下 tools：
- **query**: 执行 SQL 查询（支持只读模式）
- **profile_list**: 列出所有配置的 profiles
- **profile_show**: 查看 profile 详情

### 集成示例
在 Claude Desktop 配置中添加：
```json
{
  "mcpServers": {
    "xsql": {
      "command": "xsql",
      "args": ["mcp", "server", "--config", "/path/to/config.yaml"]
    }
  }
}
```

### 详细规范
详见 `docs/cli-spec.md` 中的 `xsql mcp server` 命令说明。

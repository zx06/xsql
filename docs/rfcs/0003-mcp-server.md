# RFC 0003: MCP Server 实现

Status: Draft

## 摘要
为 xsql 实现 MCP (Model Context Protocol) Server，提供数据库查询能力作为 MCP tools，使 AI 助手能够通过标准 MCP 协议访问 MySQL/PostgreSQL 数据库。MCP Server 将复用现有的配置、db、ssh、输出等核心能力，保持与 CLI 的一致性和兼容性。

## 背景 / 动机
- 当前痛点：AI 助手（如 Claude Desktop、Cursor 等）需要通过标准协议访问外部工具和数据源
- 目标：让 xsql 作为 MCP server 提供数据库查询能力，使 AI 助手能够直接执行 SQL 查询并获取结构化结果
- 非目标：
  - 不改变 xsql CLI 的现有行为和输出格式
  - 不实现 MCP 协议的完整特性（如 prompts、resources），仅实现 tools

## 方案（Proposed）
### 用户视角（CLI/配置）
- 新增命令：`xsql mcp server` 启动 MCP server
- 新增配置项（可选，在配置文件中）：
  ```yaml
  mcp:
    enabled: true
    transport: stdio  # 默认使用 stdio 传输
  ```
- MCP Server 提供的 tools：
  - `query`: 执行 SQL 查询（支持只读模式）
  - `profile_list`: 列出所有 profiles
  - `profile_show`: 查看 profile 详情

### 技术设计（Architecture）
- 涉及模块：
  - `internal/mcp`: MCP server 核心实现
    - `server.go`: MCP server 主逻辑
    - `transport.go`: stdio 传输实现
    - `tools.go`: MCP tools 定义和处理
  - `cmd/xsql/mcp`: MCP server CLI 命令入口
- 数据结构/接口：
  - MCP Protocol: 遵循 [Model Context Protocol](https://modelcontextprotocol.io/) 规范
  - Tool 输入/输出：复用 xsql 现有的 JSON 输出格式（`ok`、`schema_version`、`data`/`error`）
- 兼容性策略：
  - 新增能力，不影响 CLI 现有功能
  - MCP server 使用独立的代码路径，复用 internal 层的核心能力

### MCP Tools 设计
1. **query_tool**:
   ```json
   {
     "name": "query",
     "description": "Execute SQL query on database",
     "inputSchema": {
       "type": "object",
       "properties": {
         "sql": {"type": "string", "description": "SQL query to execute"},
         "profile": {"type": "string", "description": "Profile name to use"}
       },
       "required": ["sql", "profile"]
     }
   }
   ```

2. **profile_list_tool**:
   ```json
   {
     "name": "profile_list",
     "description": "List all configured profiles",
     "inputSchema": {
       "type": "object",
       "properties": {}
     }
   }
   ```

3. **profile_show_tool**:
   ```json
   {
     "name": "profile_show",
     "description": "Show profile details",
     "inputSchema": {
       "type": "object",
       "properties": {
         "name": {"type": "string", "description": "Profile name"}
       },
       "required": ["name"]
     }
   }
   ```

## 备选方案（Alternatives）
- 方案 A：实现独立的 MCP server 进程，通过 HTTP 通信
  - 优点：解耦更彻底，支持远程访问
  - 缺点：增加部署复杂度，不符合 MCP 的本地工具定位
- 方案 B：只实现 query tool，不提供 profile 管理
  - 优点：实现简单
  - 缺点：用户体验差，无法动态切换 profile

## 兼容性与迁移
- 不破坏兼容：这是新增功能，不影响现有 CLI
- 迁移步骤：无需迁移，MCP server 是可选功能
- deprecation 计划：无

## 安全与隐私
- 默认安全策略：
  - query tool 默认只读模式（双重保护：SQL 静态分析 + DB 事务级只读）
  - 写操作需要显式设置 `unsafe_allow_write: true`
- secrets 管理：
  - 复用现有的 secret 管理（keyring/加密/明文兼容）
  - 不在 MCP tool 输出中暴露密码、私钥等敏感信息

## 测试计划
### 单元测试
- MCP server 消息解析和序列化
- Tool 定义验证
- 输入参数校验
- 错误码映射

### 集成测试
- MCP server 启动和通信（stdio）
- Tool 调用流程（tools/list、tools/call）
- 复用现有数据库集成测试

### 端到端测试
- 使用 MCP client 连接 server 并调用 tools
- 验证查询结果格式和错误处理
- 验证只读保护机制

## 未决问题
- 是否需要支持 HTTP transport（除 stdio 外）？
- 是否需要实现 MCP 的 resources 和 prompts 功能？
- MCP server 是否需要支持多客户端并发？
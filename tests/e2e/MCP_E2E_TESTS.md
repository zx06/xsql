# MCP E2E 测试文档

## 概述

本文档描述 xsql MCP（Model Context Protocol）服务器功能的端到端（E2E）测试。

## 测试覆盖范围

### 1. 服务器初始化测试

- **TestMCPServer_Startup**: 验证 MCP 服务器正确启动并暴露所有工具
- **TestMCPServer_EnumValidation**: 验证枚举中的配置文件名称
- **TestMCPServer_QueryToolEnum**: 检查查询工具的枚举配置是否正确
- **TestMCPServer_EmptyConfig**: 测试空配置下的服务器行为

### 2. MCP 查询工具测试

#### 基础功能
- **TestMCPQuery_MySQL_BasicSelect**: 测试 MySQL 基本 SELECT 查询执行
- **TestMCPQuery_MySQL_MultipleRows**: 验证多行结果的处理
- **TestMCPQuery_PG_BasicSelect**: 测试 PostgreSQL 查询执行
- **TestMCPQuery_DataTypes**: 验证各种数据类型（整数、浮点数、字符串、布尔值、空值）

#### 错误处理
- **TestMCPQuery_MySQL_InvalidSQL**: 测试无效 SQL 的错误处理
- **TestMCPQuery_MissingSQL**: 验证缺少 SQL 参数时的错误
- **TestMCPQuery_MissingProfile**: 验证缺少配置文件参数时的错误
- **TestMCPQuery_ProfileNotFound**: 测试指定配置文件不存在时的错误

#### 安全与只读保护
- **TestMCPQuery_MySQL_ReadOnlyBlocked**: 验证写入操作被正确拦截：
  - INSERT 语句
  - UPDATE 语句
  - DELETE 语句
  - CREATE TABLE 语句
  - DROP TABLE 语句

### 3. 配置文件管理测试

- **TestMCPProfileList**: 测试 profile_list 工具功能
  - 验证所有配置文件都被列出
  - 检查 mode 字段（只读 vs 读写）
  - 验证配置文件元数据（名称、描述、数据库类型）

- **TestMCPProfileShow**: 测试 profile_show 工具
  - 验证返回配置文件详情
  - 确认密码脱敏（显示"***"）
  - 确保敏感数据不会泄露

- **TestMCPProfileShow_NotFound**: 测试不存在的配置文件的错误处理

### 4. SSH 代理测试

- **TestMCPQuery_SSHProxy_ConfigurationHandling**: 验证 SSH 代理配置被正确处理
- **TestMCPQuery_SSHProxy_InvalidConfiguration**: 测试无效的 SSH 代理配置错误
- **TestMCPQuery_SSHProxy_MissingIdentityFile**: 测试缺失 SSH 私钥文件的错误
- **TestMCPProfileShow_WithSSHProxy**: 验证 profile_show 包含 SSH 代理信息
- **TestMCPProfileList_WithSSHProxy**: 测试列出带 SSH 代理的配置文件
- **TestMCPQuery_SSHProxy_PassphraseHandling**: 测试 SSH 密钥密码处理
- **TestMCPQuery_SSHProxy_HostKeyValidation**: 测试 SSH 主机密钥验证

## 测试架构

### 辅助函数

#### `callMCPTool(t, configPath, toolName, arguments) (*mcp.CallToolResult, error)`
执行 MCP 工具调用：
1. 启动 MCP 服务器进程
2. 发送初始化请求
3. 调用指定的工具并传入参数
4. 解析并返回结果

#### `listMCPTools(t, configPath) []mcp.Tool`
从运行中的服务器列出所有可用的 MCP 工具。

### 测试数据结构

所有测试都使用 `createTempConfig(t, yamlContent)` 创建的临时配置文件。

### 响应格式

MCP 工具响应遵循以下 JSON 结构：

```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    // 工具特定的响应数据
  }
}
```

错误响应：

```json
{
  "ok": false,
  "schema_version": 1,
  "error": {
    "code": "XSQL_...",
    "message": "错误描述",
    "details": {
      // 可选的错误详情
    }
  }
}
```

## 运行测试

### 前置条件

测试需要：
- MySQL 服务器（用于 MySQL 测试）
- PostgreSQL 服务器（用于 PostgreSQL 测试）

设置环境变量：
```bash
export XSQL_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/testdb"
export XSQL_TEST_PG_DSN="postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable"
```

### 运行所有 E2E 测试

```bash
go test -v -tags=e2e ./tests/e2e/...
```

### 运行特定测试

```bash
go test -v -tags=e2e ./tests/e2e/... -run TestMCPQuery_MySQL_BasicSelect
```

### 带覆盖率运行

```bash
go test -v -tags=e2e -coverprofile=coverage-e2e.txt -covermode=atomic ./tests/e2e/...
```

## CI/CD 集成

E2E 测试在 CI 中自动运行：
- MySQL 8.0 服务容器
- PostgreSQL 16 服务容器

详见 `.github/workflows/ci.yml`。

## 错误码覆盖

| 错误码 | 描述 | 测试覆盖 |
|------|-------------|---------------|
| XSQL_CFG_INVALID | 配置或参数无效 | ✅ 已覆盖 |
| XSQL_RO_BLOCKED | 只读模式拦截写入操作 | ✅ 已覆盖 |
| XSQL_DB_EXEC_FAILED | 数据库执行错误 | ✅ 已覆盖 |
| XSQL_SSH_DIAL_FAILED | SSH 连接失败 | ✅ 已覆盖 |
| XSQL_SSH_AUTH_FAILED | SSH 认证失败 | ✅ 已覆盖 |
| XSQL_SECRET_INVALID | 密钥格式无效 | ✅ 已覆盖 |

## 测试场景覆盖矩阵

| 场景 | MySQL | PostgreSQL | 状态 |
|----------|-------|------------|--------|
| 基本 SELECT 查询 | ✅ | ✅ | 完成 |
| 多行结果 | ✅ | ✅ | 完成 |
| 数据类型 | ✅ | ✅ | 完成 |
| 无效 SQL | ✅ | ❌ | 仅 MySQL |
| 只读保护 | ✅ | ❌ | 仅 MySQL |
| 缺少参数 | ✅ | ✅ | 完成 |
| 配置文件不存在 | ✅ | ✅ | 完成 |
| 配置文件列表 | ✅ | ✅ | 完成 |
| 配置文件详情 | ✅ | ✅ | 完成 |
| 密码脱敏 | ✅ | ✅ | 完成 |
| SSH 代理配置 | ✅ | ❌ | 仅 MySQL |
| SSH 代理错误处理 | ✅ | ❌ | 仅 MySQL |

## 调试失败的测试

### 常见问题

1. **数据库连接失败**
   - 验证 XSQL_TEST_MYSQL_DSN / XSQL_TEST_PG_DSN 已设置
   - 检查数据库服务是否运行
   - 验证网络连通性

2. **MCP 协议错误**
   - 检查 MCP 服务器二进制是否正确构建
   - 验证 MCP SDK 版本兼容性
   - 查看 stderr 输出中的服务器错误

3. **超时问题**
   - 增加 `callMCPTool` 中的缓冲区大小
   - 检查数据库查询性能
   - 查看系统资源使用情况

### 详细调试

启用详细测试输出：
```bash
go test -v -tags=e2e ./tests/e2e/... -test.v
```

如有需要，在测试辅助函数中添加调试日志。

## 贡献指南

添加新的 MCP E2E 测试时：

1. 遵循现有命名规范：`TestMCP[功能]_[数据库]_[场景]`
2. 使用辅助函数（`callMCPTool`, `createTempConfig`）
3. 测试成功和错误两种情况
4. 验证错误码和错误消息
5. 更新本文档
6. 确保测试是幂等的（可以多次运行）

## 相关文档

- [MCP 服务器实现](../../internal/mcp/tools.go)
- [E2E 测试基础设施](./e2e_test.go)
- [CLI E2E 测试](./e2e_test.go)
- [集成测试](../integration/)

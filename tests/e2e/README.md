# E2E 测试

xsql CLI 和 MCP 服务器的端到端测试。

## 快速开始

### 1. 启动测试数据库

使用 Docker Compose：
```bash
docker-compose up -d
```

或手动启动：
```bash
# MySQL
docker run -d --name xsql-mysql \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=testdb \
  -p 3306:3306 \
  mysql:8.0

# PostgreSQL
docker run -d --name xsql-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  postgres:16
```

### 2. 设置环境变量

```bash
export XSQL_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/testdb"
export XSQL_TEST_PG_DSN="postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable"
```

### 3. 运行测试

```bash
# 所有 E2E 测试
go test -v -tags=e2e ./tests/e2e/...

# 特定测试
go test -v -tags=e2e ./tests/e2e/... -run TestMCPQuery_MySQL_BasicSelect

# 带覆盖率
go test -v -tags=e2e -coverprofile=coverage.txt ./tests/e2e/...
```

## 测试分类

### CLI 测试 (`e2e_test.go`)
- 基础查询执行
- 输出格式（JSON、YAML、表格、CSV）
- 错误处理
- 配置文件管理
- 只读保护

### MCP 测试 (`mcp_test.go`)
- MCP 服务器初始化
- MCP 查询工具执行
- MCP 配置文件管理
- MCP 协议错误处理
- SSH 代理测试

### 配置文件测试 (`profile_test.go`)
- 配置文件列表
- 配置文件详情
- 配置文件验证

## 环境变量

| 变量 | 描述 | 示例 |
|----------|-------------|---------|
| `XSQL_TEST_MYSQL_DSN` | MySQL 连接字符串 | `root:root@tcp(127.0.0.1:3306)/testdb` |
| `XSQL_TEST_PG_DSN` | PostgreSQL 连接字符串 | `postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable` |

## 调试

### 查看测试输出
```bash
go test -v -tags=e2e ./tests/e2e/... 2>&1 | tee test.log
```

### 运行单个测试并显示详细信息
```bash
go test -v -tags=e2e ./tests/e2e/... -run TestName -test.v
```

### 检查数据库连接
```bash
# MySQL
mysql -h 127.0.0.1 -u root -proot testdb -e "SELECT 1"

# PostgreSQL
psql "postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable" -c "SELECT 1"
```

## 常见问题

### 端口已被占用
如果端口 3306 或 5432 已被占用：
```bash
# 停止现有容器
docker stop xsql-mysql xsql-postgres
docker rm xsql-mysql xsql-postgres

# 或使用不同端口
docker run -d --name xsql-mysql -p 13306:3306 ...
```

### 数据库未就绪
等待数据库就绪：
```bash
# MySQL
until mysql -h 127.0.0.1 -u root -proot -e "SELECT 1" &>/dev/null; do
  echo "Waiting for MySQL..."
  sleep 2
done

# PostgreSQL
until psql "postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable" -c "SELECT 1" &>/dev/null; do
  echo "Waiting for PostgreSQL..."
  sleep 2
done
```

## CI/CD

测试在 GitHub Actions 中自动运行：
- MySQL 8.0 服务
- PostgreSQL 16 服务

详见 `.github/workflows/ci.yml`。

## 更多信息

- [MCP E2E 测试文档](./MCP_E2E_TESTS.md)
- [测试架构](./e2e_test.go)

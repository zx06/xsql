# 开发指南（Dev Guide）

## 基本原则
- 机器输出（json/yaml）只写 stdout；日志/调试信息只写 stderr。
- 核心逻辑放 `internal/*`，CLI 仅做参数解析与输出。
- 为命令/输出/错误码维护稳定契约（AI-first）。

## 依赖
- CLI：cobra
- Config：自研（支持 CLI > ENV > Config 优先级合并）
- DB：database/sql + go-sql-driver/mysql + jackc/pgx
- Secrets：zalando/go-keyring
- SSH：x/crypto/ssh
- Output：json/yaml/csv + text/tabwriter

## 快速开始

```bash
# 构建
go build -o xsql ./cmd/xsql

# 运行单元测试
go test ./...

# 运行测试（带覆盖率）
go test -cover ./...

# 导出 spec
./xsql spec --format json
```

## 集成测试

集成测试需要 MySQL 和 PostgreSQL 数据库。使用 docker-compose 启动：

```bash
# 启动数据库
docker-compose up -d

# 等待数据库就绪（约 10-20 秒）
docker-compose ps

# 运行集成测试
XSQL_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/testdb?parseTime=true" \
XSQL_TEST_PG_DSN="postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable" \
go test -v -tags=integration ./tests/integration/...

# 停止数据库
docker-compose down
```

### Windows PowerShell

```powershell
# 启动数据库
docker-compose up -d

# 运行集成测试
$env:XSQL_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/testdb?parseTime=true"
$env:XSQL_TEST_PG_DSN="postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable"
go test -v -tags=integration ./tests/integration/...

# 停止数据库
docker-compose down
```

### 集成测试覆盖范围

MySQL 测试：
- 基本查询（SELECT）
- 多行结果、空结果
- 数据类型（int, float, string, bool, datetime）
- SHOW DATABASES、EXPLAIN
- 只读策略（INSERT/UPDATE/DELETE/DROP/CREATE 拦截）
- 无效 SQL 错误处理

PostgreSQL 测试：
- 基本查询
- generate_series、CTE
- 系统表查询（pg_database）
- 只读策略
- JSON 数据类型

## 目录结构

```
cmd/xsql/          # CLI 入口
internal/
  app/             # 应用编排
  config/          # 配置加载/合并/校验 + profile
  db/              # driver registry + query
    mysql/         # MySQL driver
    pg/            # PostgreSQL driver
  errors/          # 错误码/退出码
  log/             # slog 日志
  output/          # json/yaml/table/csv 输出
  secret/          # keyring + 明文策略
  spec/            # tool spec 导出
  ssh/             # SSH client + dial
tests/
  integration/     # 集成测试（需要数据库）
docs/              # 文档
```

## 添加新 DB driver

1. 在 `internal/db/<name>/` 创建 driver 实现
2. 实现 `db.Driver` 接口
3. 在 `init()` 中调用 `db.Register("<name>", &Driver{})`
4. 在 `cmd/xsql/main.go` 中 import 触发注册

## CI/CD

项目使用 GitHub Actions 进行 CI：

- 单元测试（所有平台）
- 集成测试（MySQL 8.0 + PostgreSQL 16）
- 代码检查（golangci-lint）
- 代码覆盖率上报

详见 `.github/workflows/ci.yml`。

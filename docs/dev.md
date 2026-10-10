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
# 构建 web UI
npm --prefix webui install
npm --prefix webui run build

# 前端联调（默认代理到 http://127.0.0.1:8788）
xsql serve --addr 127.0.0.1:8788
npm --prefix webui run dev

# 构建
go build -o xsql ./cmd/xsql

# 运行单元测试
go test ./...

# 运行测试（带覆盖率）
go test -cover ./...

# 运行 E2E 测试
go test -tags=e2e ./tests/e2e/...

# 导出 spec
./xsql spec --format json
```

> 详细测试指南见 [docs/testing.md](testing.md)

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
  ai/              # AI LLM 客户端与 Prompt 组装
  app/             # 应用编排
  config/          # 配置加载/合并/校验 + profile
  db/              # driver registry + query
    mysql/         # MySQL driver
    pg/            # PostgreSQL driver
  errors/          # 错误码/退出码
  export/          # AI 会话数据导出
  js/              # goja 数据分析沙箱
  log/             # slog 日志
  mcp/             # MCP Server 实现
  output/          # json/yaml/table/csv 输出
  proxy/           # 端口转发代理
  secret/          # keyring + 明文策略
  session/         # AI 会话数据集存储
  spec/            # tool spec 导出
  ssh/             # SSH client + dial
  stats/           # 使用统计与 attr 聚合
  tui/             # AI 交互式终端 UI
  web/             # Web API 与静态资源适配
webui/             # Svelte 前端源码 + 嵌入资源
tests/
  e2e/             # E2E 测试
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
- 代码覆盖率上报（仓库内 PR / `main` 推送在配置了 `CODECOV_TOKEN` 时上传；fork / Dependabot PR 无 token 时会跳过上传，且不会因 Codecov 失败而阻塞）

详见 `.github/workflows/ci.yml`。

### 发布与发布前检查

正式发布由 tag 推送触发（`.github/workflows/release.yml`），顺序为：

1. **GoReleaser snapshot 构建**（`goreleaser build --snapshot`），产出各平台二进制到 `dist/` 并上传 artifact。
2. **冒烟测试**（`.github/workflows/release-smoke-reusable.yml`）：
   - 二进制：在 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 上运行 `xsql version`、`xsql spec`、`xsql profile list`（脚本：`scripts/release-smoke.js binary`）。
   - npm：在 Linux / macOS / Windows 上从 `dist/` 填充平台包、`npm pack` 后本地安装 tarball，执行 `npx xsql version` 与 `npx xsql spec`（`scripts/release-smoke.js npm`）。
3. **GoReleaser release**（`goreleaser release --clean`）——仅在上一步全部通过后创建 GitHub Release 并推送 Homebrew/Scoop 等。
4. **npm 发布**（`scripts/npm-publish.js`）——从 GitHub Release 下载二进制后发布 `xsql-cli` 与各 `@zx06/xsql-cli-*` 平台包。

修改发布相关文件时，PR 会触发 **Release dry run**（`.github/workflows/release-dry-run.yml`）：`actionlint` 校验 workflow、snapshot 构建与上述冒烟测试（不发布）。也可在 Actions 中手动 **workflow_dispatch** 该 workflow。

本地可复现 snapshot + 冒烟：

```bash
goreleaser build --snapshot --clean
node scripts/release-smoke.js binary dist linux amd64
node scripts/release-smoke.js npm dist
```

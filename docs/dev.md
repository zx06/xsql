# 开发指南（Dev Guide）

## 基本原则
- 机器输出（json/yaml/csv）只写 stdout；日志/调试信息只写 stderr。
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

# 运行测试
go test ./...

# 导出 spec
./xsql spec --format json
```

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
```

## 添加新 DB driver

1. 在 `internal/db/<name>/` 创建 driver 实现
2. 实现 `db.Driver` 接口
3. 在 `init()` 中调用 `db.Register("<name>", &Driver{})`
4. 在 `cmd/xsql/main.go` 中 import 触发注册

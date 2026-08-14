# RFC 0008: Usage Stats & Global Attributes

Status: Implemented

## 摘要

为 xsql 增加使用统计功能，记录每次命令执行的元数据（命令、profile、成功/失败、耗时），支持可选的 SQL 内容记录用于审计。同时引入全局 `--attr` flag，允许用户在执行命令时标记自定义属性（如 `env=prod`、`team=ai`），从而支持按任意维度进行聚合统计。

此功能默认关闭，不改变任何现有行为。

## 背景 / 动机

- 当前痛点：无法追踪 xsql 的使用情况，无法按维度分析使用模式，AI agent 场景需要审计调用记录
- 目标：记录命令执行元数据、支持自定义属性、提供聚合统计查询
- 非目标：遥测系统、性能监控、记录 profile 配置内容

## 方案（Proposed）

### 用户视角（CLI/配置/输出）

**全局 `--attr` flag：**

```bash
xsql query "SELECT * FROM users" -p prod --attr source=codex-cli --attr agent=codex --attr env=prod --attr task=targeted-query
export XSQL_ATTR='source=codex-cli,agent=codex,env=prod,team=data'
xsql query "SELECT * FROM users" -p prod
```

Codex 发起的 CLI 调用统一附带 `--attr source=codex-cli`，并可继续追加 `agent`、`env`、`team`、`task` 等属性；`source` 不应被覆盖。其他 AI 客户端应使用自己的稳定 source 值。

**stats 命令：**

```bash
xsql stats                        # 聚合报告
xsql stats --profile prod-mysql   # 按 profile 过滤
xsql stats --json                 # JSON 输出
xsql stats log --limit 100        # 明细查询
xsql stats reset                  # 重置统计
```

**配置项：**

```yaml
stats:
  enabled: true
  log_sql: false
  file_path: ~/.config/xsql/stats.jsonl
  retention_days: 30
```

**ENV 变量：**

```bash
XSQL_STATS_ENABLED=true
XSQL_STATS_LOG_SQL=true
XSQL_ATTR='source=codex-cli,agent=codex,env=prod,team=data,task=health-check'
```

### 技术设计（Architecture）

涉及模块：

```
internal/stats/           # 核心模块
  types.go                #   Record、StatsConfig 结构体
  attrs.go                #   Attribute 解析
  store.go                #   JSONL 追加写入
  query.go                #   聚合查询

cmd/xsql/
  root.go                 #   增加 --attr persistent flag
  stats.go                #   stats 命令
  query.go                #   集成统计记录
  schema.go               #   集成统计记录
```

数据结构：

```go
type Record struct {
    Timestamp  time.Time         `json:"ts"`
    Cmd        string            `json:"cmd"`
    Profile    string            `json:"profile"`
    OK         bool              `json:"ok"`
    DurationMs int64             `json:"duration_ms"`
    ErrorCode  string            `json:"error_code,omitempty"`
    SQL        string            `json:"sql,omitempty"`
    Attrs      map[string]string `json:"attrs,omitempty"`
}

type StatsConfig struct {
    Enabled       bool   `yaml:"enabled" json:"enabled"`
    LogSQL        bool   `yaml:"log_sql" json:"log_sql"`
    FilePath      string `yaml:"file_path" json:"file_path,omitempty"`
    RetentionDays int    `yaml:"retention_days" json:"retention_days,omitempty"`
}

type AggregatedRecord struct {
    Cmd     string            `json:"cmd"`
    Profile string            `json:"profile"`
    Attrs   map[string]string `json:"attrs,omitempty"`
    OK      int               `json:"ok"`
    Fail    int               `json:"fail"`
    AvgMs   int64             `json:"avg_ms"`
}
```

并发写入策略：CLI 用 O_APPEND 原子追加；MCP/Web 长驻进程用 sync.Mutex 保护。

兼容性策略：只增不改，默认关闭。

## 备选方案（Alternatives）

- 方案 A：SQLite — 更强查询但引入新依赖，不采用
- 方案 B：CSV — 不支持嵌套 attrs，不采用
- 方案 C：存目标数据库 — 依赖数据库连接且违反只读策略，不采用

## 兼容性与迁移（Compatibility & Migration）

- 是否破坏兼容：否
- 迁移步骤：无需迁移，新功能默认关闭
- deprecation 计划：无

## 安全与隐私（Security/Privacy）

- secrets 暴露风险：sql 字段默认不记录，统计文件权限 0600
- 默认安全策略：统计默认关闭，SQL 内容默认不记录

## 测试计划（Test Plan）

- 单元测试：attrs 解析、store 追加写入、query 聚合
- 集成测试：stats 命令输出、--attr 传递验证

## 未决问题（Open Questions）

- 无

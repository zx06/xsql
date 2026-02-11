# RFC 0005: Schema Dump

Status: Implemented

## 摘要
新增 `xsql schema dump` 命令，导出数据库结构信息（表、列、类型、约束、索引），供 AI/agent 自动理解数据库 schema。输出为结构化 JSON/YAML，遵循 xsql 标准输出契约。

## 背景 / 动机
- **当前痛点**：AI 使用 xsql 查询数据库时，需要人工提供表结构信息，否则无法知道有哪些表、字段类型是什么。
- **目标**：让 AI 能通过 `xsql schema dump` 自动发现数据库结构，无需人工介入。
- **非目标**：
  - 不支持修改 schema（只读）
  - 不支持导出数据内容（仅结构）
  - 不支持视图定义、存储过程、触发器（v1 版本）

## 方案（Proposed）

### 用户视角（CLI/配置/输出）

#### 新增命令
```bash
xsql schema dump -p <profile> [-f json|yaml|table] [--table pattern] [--include-system]
```

#### Flags
| Flag | 默认值 | 说明 |
|------|--------|------|
| `-p, --profile` | 必填 | Profile 名称 |
| `-f, --format` | auto | 输出格式：json/yaml/table/auto |
| `--table` | "" | 表名过滤模式（支持通配符 `*` 和 `?`） |
| `--include-system` | false | 是否包含系统表（如 `information_schema`、`pg_catalog`） |

#### 输出结构（JSON）
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "database": "mydb",
    "tables": [
      {
        "schema": "public",
        "name": "users",
        "comment": "用户表",
        "columns": [
          {
            "name": "id",
            "type": "bigint",
            "nullable": false,
            "default": "nextval('users_id_seq'::regclass)",
            "comment": "主键",
            "primary_key": true
          },
          {
            "name": "email",
            "type": "varchar(255)",
            "nullable": false,
            "default": null,
            "comment": "邮箱",
            "primary_key": false
          }
        ],
        "indexes": [
          {
            "name": "users_pkey",
            "columns": ["id"],
            "unique": true,
            "primary": true
          },
          {
            "name": "users_email_idx",
            "columns": ["email"],
            "unique": true,
            "primary": false
          }
        ],
        "foreign_keys": [
          {
            "name": "orders_user_id_fkey",
            "columns": ["user_id"],
            "referenced_table": "users",
            "referenced_columns": ["id"]
          }
        ]
      }
    ]
  }
}
```

#### Table 格式（人类可读）
```
Table: public.users (用户表)
  Columns:
    name        type          nullable  default             comment  primary_key
    ----        ----          --------  -------             -------  -----------
    id          bigint        false     nextval('users...') 主键     true
    email       varchar(255)  false     null                邮箱     false
    created_at  timestamp     true      now()               创建时间 false

  Indexes:
    name              columns   unique  primary
    ----              -------   ------  -------
    users_pkey        id        true    true
    users_email_idx   email     true    false

  Foreign Keys:
    name                 columns   referenced_table  referenced_columns
    ----                 -------   ----------------  ------------------
    orders_user_id_fkey  user_id   users             id
```

### 技术设计（Architecture）

#### 涉及模块
- `internal/db/schema.go` - schema 提取核心逻辑
- `internal/db/mysql/schema.go` - MySQL 实现
- `internal/db/pg/schema.go` - PostgreSQL 实现
- `internal/db/registry.go` - 扩展 Driver 接口（可选）
- `cmd/xsql/schema.go` - CLI 命令
- `docs/cli-spec.md` - 文档更新

#### 数据结构
```go
// SchemaInfo 数据库 schema 信息
type SchemaInfo struct {
    Database string  `json:"database" yaml:"database"`
    Tables   []Table `json:"tables" yaml:"tables"`
}

// Table 表信息
type Table struct {
    Schema      string       `json:"schema" yaml:"schema"`           // PostgreSQL schema
    Name        string       `json:"name" yaml:"name"`               // 表名
    Comment     string       `json:"comment,omitempty" yaml:"comment,omitempty"`
    Columns     []Column     `json:"columns" yaml:"columns"`
    Indexes     []Index      `json:"indexes,omitempty" yaml:"indexes,omitempty"`
    ForeignKeys []ForeignKey `json:"foreign_keys,omitempty" yaml:"foreign_keys,omitempty"`
}

// Column 列信息
type Column struct {
    Name       string `json:"name" yaml:"name"`
    Type       string `json:"type" yaml:"type"`
    Nullable   bool   `json:"nullable" yaml:"nullable"`
    Default    string `json:"default,omitempty" yaml:"default,omitempty"`
    Comment    string `json:"comment,omitempty" yaml:"comment,omitempty"`
    PrimaryKey bool   `json:"primary_key" yaml:"primary_key"`
}

// Index 索引信息
type Index struct {
    Name    string   `json:"name" yaml:"name"`
    Columns []string `json:"columns" yaml:"columns"`
    Unique  bool     `json:"unique" yaml:"unique"`
    Primary bool     `json:"primary" yaml:"primary"`
}

// ForeignKey 外键信息
type ForeignKey struct {
    Name              string   `json:"name" yaml:"name"`
    Columns           []string `json:"columns" yaml:"columns"`
    ReferencedTable   string   `json:"referenced_table" yaml:"referenced_table"`
    ReferencedColumns []string `json:"referenced_columns" yaml:"referenced_columns"`
}
```

#### Driver 接口扩展（可选方案）
```go
// SchemaDriver 扩展接口（可选实现）
type SchemaDriver interface {
    Driver
    // DumpSchema 导出数据库结构
    DumpSchema(ctx context.Context, db *sql.DB, opts SchemaOptions) (*SchemaInfo, *errors.XError)
}

// SchemaOptions schema 导出选项
type SchemaOptions struct {
    TablePattern  string // 表名过滤
    IncludeSystem bool   // 包含系统表
}
```

#### MySQL 实现策略
使用 `information_schema` 查询：
```sql
-- 表信息
SELECT table_schema, table_name, table_comment
FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE';

-- 列信息
SELECT column_name, data_type, column_type, is_nullable, 
       column_default, column_comment, column_key
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ?;

-- 索引信息
SELECT index_name, column_name, non_unique, index_name = 'PRIMARY'
FROM information_schema.statistics
WHERE table_schema = DATABASE() AND table_name = ?;

-- 外键信息
SELECT constraint_name, column_name, referenced_table_name, referenced_column_name
FROM information_schema.key_column_usage
WHERE table_schema = DATABASE() AND table_name = ? 
  AND referenced_table_name IS NOT NULL;
```

#### PostgreSQL 实现策略
使用 `information_schema` + `pg_catalog`：
```sql
-- 表信息
SELECT schemaname, tablename, obj_description((schemaname || '.' || tablename)::regclass) as comment
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema');

-- 列信息
SELECT column_name, data_type, udt_name, is_nullable, column_default,
       col_description((table_schema || '.' || table_name)::regclass, ordinal_position)
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2;

-- 索引信息（使用 pg_indexes + pg_index）
SELECT indexname, indexdef
FROM pg_indexes
WHERE schemaname = $1 AND tablename = $2;

-- 外键信息
SELECT constraint_name, column_name, referenced_table_name, referenced_column_name
FROM information_schema.key_column_usage
WHERE table_schema = $1 AND table_name = $2
  AND referenced_table_name IS NOT NULL;
```

#### 兼容性策略
- **只增不改**：新增命令，不影响现有命令
- **可选实现**：SchemaDriver 为扩展接口，driver 可选择不实现（返回错误码 `XSQL_UNSUPPORTED`）
- **版本化**：输出结构包含 `schema_version`，未来可扩展字段

## 备选方案（Alternatives）

### 方案 A：独立命令 `xsql schema`
```bash
xsql schema -p dev          # 等价于 xsql schema dump
xsql schema tables -p dev   # 只列出表名
xsql schema columns users -p dev  # 只列出某表的列
```
**优点**：更细粒度控制
**缺点**：增加复杂度，v1 不需要

### 方案 B：作为 query 的特殊语法
```bash
xsql query "DESCRIBE SCHEMA" -p dev
```
**优点**：复用现有命令
**缺点**：不符合 SQL 语义，混淆查询和元数据

### 方案 C：MCP Tool 独立提供
只在 MCP Server 中提供 schema tool，不暴露 CLI 命令。
**优点**：减少 CLI 复杂度
**缺点**：非 MCP 用户无法使用

**选择**：采用主方案（`xsql schema dump`），v1 保持简单，未来可扩展为方案 A。

## 兼容性与迁移（Compatibility & Migration）
- **是否破坏兼容**：否，纯新增功能
- **迁移步骤**：无需迁移
- **deprecation 计划**：无

## 安全与隐私（Security/Privacy）
- **secrets 暴露风险**：无，schema 信息不包含敏感数据
- **默认安全策略**：
  - 默认不导出系统表（避免暴露数据库内部结构）
  - 遵循 profile 的只读策略（schema dump 本质是查询 information_schema）
  - 表注释可能包含业务信息，由用户自行负责

## 测试计划（Test Plan）

### 单元测试
- `internal/db/schema_test.go`：表名过滤逻辑、输出序列化
- `internal/db/mysql/schema_test.go`：MySQL information_schema 结果解析
- `internal/db/pg/schema_test.go`：PostgreSQL 结果解析

### 集成测试
- `tests/integration/schema_test.go`：
  - MySQL 真实数据库 schema 导出
  - PostgreSQL 真实数据库 schema 导出
  - 表名过滤功能
  - 系统表排除功能

### E2E 测试
- `tests/e2e/schema_test.go`：
  - JSON 输出格式验证
  - YAML 输出格式验证
  - Table 输出格式验证
  - 错误场景（profile 不存在、连接失败）

## 未决问题（Open Questions）

1. **是否支持视图（VIEW）？**
   - v1 不支持，后续可通过 `--include-views` flag 添加
   
2. **是否支持存储过程/函数定义？**
   - v1 不支持，安全风险较高（可能包含敏感逻辑）

3. **大数据库性能？**
   - 如果数据库有数千张表，输出可能很大
   - 解决方案：`--table` 过滤 + 流式输出（未来）

4. **是否需要 `xsql schema diff` 对比两个环境的 schema？**
   - 有价值，但 v1 不做，作为后续增强
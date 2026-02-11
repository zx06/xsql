package db

import (
	"context"
	"database/sql"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

// SchemaInfo 数据库 schema 信息
type SchemaInfo struct {
	Database string  `json:"database" yaml:"database"`
	Tables   []Table `json:"tables" yaml:"tables"`
}

// ToSchemaData 实现 output.SchemaFormatter 接口
func (s *SchemaInfo) ToSchemaData() (string, []output.SchemaTable, bool) {
	if s == nil || len(s.Tables) == 0 {
		return "", nil, false
	}

	tables := make([]output.SchemaTable, len(s.Tables))
	for i, t := range s.Tables {
		tables[i].Schema = t.Schema
		tables[i].Name = t.Name
		tables[i].Comment = t.Comment
		tables[i].Columns = make([]output.SchemaColumn, len(t.Columns))
		for j, c := range t.Columns {
			tables[i].Columns[j] = output.SchemaColumn{
				Name:       c.Name,
				Type:       c.Type,
				Nullable:   c.Nullable,
				Default:    c.Default,
				Comment:    c.Comment,
				PrimaryKey: c.PrimaryKey,
			}
		}
	}

	return s.Database, tables, true
}

// Table 表信息
type Table struct {
	Schema      string       `json:"schema" yaml:"schema"` // PostgreSQL schema，MySQL 为数据库名
	Name        string       `json:"name" yaml:"name"`     // 表名
	Comment     string       `json:"comment,omitempty" yaml:"comment,omitempty"`
	Columns     []Column     `json:"columns" yaml:"columns"`
	Indexes     []Index      `json:"indexes,omitempty" yaml:"indexes,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty" yaml:"foreign_keys,omitempty"`
}

// Column 列信息
type Column struct {
	Name       string `json:"name" yaml:"name"`
	Type       string `json:"type" yaml:"type"`                           // 数据类型，如 varchar(255)、bigint
	Nullable   bool   `json:"nullable" yaml:"nullable"`                   // 是否允许 NULL
	Default    string `json:"default,omitempty" yaml:"default,omitempty"` // 默认值
	Comment    string `json:"comment,omitempty" yaml:"comment,omitempty"` // 列注释
	PrimaryKey bool   `json:"primary_key" yaml:"primary_key"`             // 是否为主键
}

// Index 索引信息
type Index struct {
	Name    string   `json:"name" yaml:"name"`       // 索引名
	Columns []string `json:"columns" yaml:"columns"` // 索引列
	Unique  bool     `json:"unique" yaml:"unique"`   // 是否唯一索引
	Primary bool     `json:"primary" yaml:"primary"` // 是否主键索引
}

// ForeignKey 外键信息
type ForeignKey struct {
	Name              string   `json:"name" yaml:"name"`                             // 外键名
	Columns           []string `json:"columns" yaml:"columns"`                       // 本表列
	ReferencedTable   string   `json:"referenced_table" yaml:"referenced_table"`     // 引用表
	ReferencedColumns []string `json:"referenced_columns" yaml:"referenced_columns"` // 引用列
}

// SchemaOptions schema 导出选项
type SchemaOptions struct {
	TablePattern  string // 表名过滤（支持通配符）
	IncludeSystem bool   // 是否包含系统表
}

// SchemaDriver schema 导出接口
// Driver 可选择实现此接口以支持 schema 导出
type SchemaDriver interface {
	Driver
	// DumpSchema 导出数据库结构
	DumpSchema(ctx context.Context, db *sql.DB, opts SchemaOptions) (*SchemaInfo, *errors.XError)
}

// DumpSchema 导出数据库结构
// 会检查 driver 是否实现了 SchemaDriver 接口
func DumpSchema(ctx context.Context, driverName string, db *sql.DB, opts SchemaOptions) (*SchemaInfo, *errors.XError) {
	d, ok := Get(driverName)
	if !ok {
		return nil, errors.New(errors.CodeDBDriverUnsupported, "unsupported driver: "+driverName, nil)
	}

	sd, ok := d.(SchemaDriver)
	if !ok {
		return nil, errors.New(errors.CodeDBDriverUnsupported, "driver does not support schema dump: "+driverName, nil)
	}

	return sd.DumpSchema(ctx, db, opts)
}

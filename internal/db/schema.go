package db

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

// SchemaInfo represents database schema information.
type SchemaInfo struct {
	Database string  `json:"database" yaml:"database"`
	Tables   []Table `json:"tables" yaml:"tables"`
}

// TableList contains the lightweight table listing for a database.
type TableList struct {
	Database string         `json:"database" yaml:"database"`
	Tables   []TableSummary `json:"tables" yaml:"tables"`
}

// TableSummary represents a lightweight table entry for object navigation.
type TableSummary struct {
	Schema  string `json:"schema" yaml:"schema"`
	Name    string `json:"name" yaml:"name"`
	Comment string `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// ToSchemaData implements the output.SchemaFormatter interface.
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

// Table represents table information.
type Table struct {
	Schema      string       `json:"schema" yaml:"schema"` // PostgreSQL schema; database name for MySQL
	Name        string       `json:"name" yaml:"name"`     // Table name
	Comment     string       `json:"comment,omitempty" yaml:"comment,omitempty"`
	Columns     []Column     `json:"columns" yaml:"columns"`
	Indexes     []Index      `json:"indexes,omitempty" yaml:"indexes,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty" yaml:"foreign_keys,omitempty"`
}

// Column represents column information.
type Column struct {
	Name       string `json:"name" yaml:"name"`
	Type       string `json:"type" yaml:"type"`                           // Data type, e.g. varchar(255), bigint
	Nullable   bool   `json:"nullable" yaml:"nullable"`                   // Whether NULL is allowed
	Default    string `json:"default,omitempty" yaml:"default,omitempty"` // Default value
	Comment    string `json:"comment,omitempty" yaml:"comment,omitempty"` // Column comment
	PrimaryKey bool   `json:"primary_key" yaml:"primary_key"`             // Whether this is a primary key
}

// Index represents index information.
type Index struct {
	Name    string   `json:"name" yaml:"name"`       // Index name
	Columns []string `json:"columns" yaml:"columns"` // Indexed columns
	Unique  bool     `json:"unique" yaml:"unique"`   // Whether this is a unique index
	Primary bool     `json:"primary" yaml:"primary"` // Whether this is a primary key index
}

// ForeignKey represents foreign key information.
type ForeignKey struct {
	Name              string   `json:"name" yaml:"name"`                             // Foreign key name
	Columns           []string `json:"columns" yaml:"columns"`                       // Local columns
	ReferencedTable   string   `json:"referenced_table" yaml:"referenced_table"`     // Referenced table
	ReferencedColumns []string `json:"referenced_columns" yaml:"referenced_columns"` // Referenced columns
}

// SchemaOptions holds options for schema export.
type SchemaOptions struct {
	TablePattern  string // Table name filter (supports wildcards)
	IncludeSystem bool   // Whether to include system tables
}

// TableDescribeOptions identifies a single table to describe.
type TableDescribeOptions struct {
	Schema string
	Name   string
}

// SchemaExplorerDriver is the schema browsing interface.
// A Driver may optionally implement this interface to support table listing and description.
type SchemaExplorerDriver interface {
	Driver
	// ListTables returns the lightweight table list for the target database.
	ListTables(ctx context.Context, db *sql.DB, opts SchemaOptions) (*TableList, *errors.XError)
	// DescribeTable returns the schema details for a single table.
	DescribeTable(ctx context.Context, db *sql.DB, opts TableDescribeOptions) (*Table, *errors.XError)
}

// ListTables returns the lightweight table list for the target database.
func ListTables(ctx context.Context, driverName string, db *sql.DB, opts SchemaOptions) (*TableList, *errors.XError) {
	d, ok := Get(driverName)
	if !ok {
		return nil, errors.New(errors.CodeDBDriverUnsupported, "unsupported driver: "+driverName, nil)
	}

	sd, ok := d.(SchemaExplorerDriver)
	if !ok {
		return nil, errors.New(errors.CodeDBDriverUnsupported, "driver does not support schema browsing: "+driverName, nil)
	}

	result, xe := sd.ListTables(ctx, db, opts)
	if xe != nil {
		return nil, xe
	}
	if result != nil && result.Tables == nil {
		result.Tables = []TableSummary{}
	}
	return result, nil
}

// DescribeTable returns the schema details for a single table.
func DescribeTable(ctx context.Context, driverName string, db *sql.DB, opts TableDescribeOptions) (*Table, *errors.XError) {
	d, ok := Get(driverName)
	if !ok {
		return nil, errors.New(errors.CodeDBDriverUnsupported, "unsupported driver: "+driverName, nil)
	}

	sd, ok := d.(SchemaExplorerDriver)
	if !ok {
		return nil, errors.New(errors.CodeDBDriverUnsupported, "driver does not support schema browsing: "+driverName, nil)
	}

	return sd.DescribeTable(ctx, db, opts)
}

// DumpSchema exports the database schema.
// It composes the full schema dump from table listing and per-table descriptions.
func DumpSchema(ctx context.Context, driverName string, db *sql.DB, opts SchemaOptions) (*SchemaInfo, *errors.XError) {
	tableList, xe := ListTables(ctx, driverName, db, opts)
	if xe != nil {
		return nil, xe
	}

	info := &SchemaInfo{
		Database: tableList.Database,
		Tables:   make([]Table, len(tableList.Tables)),
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for i, table := range tableList.Tables {
		i := i
		table := table
		g.Go(func() error {
			detail, xe := DescribeTable(gctx, driverName, db, TableDescribeOptions{
				Schema: table.Schema,
				Name:   table.Name,
			})
			if xe != nil {
				return xe
			}
			info.Tables[i] = *detail
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, errors.AsOrWrap(err)
	}

	return info, nil
}

// PopulateTableDetails concurrently loads columns, indexes, and foreign keys for a table.
func PopulateTableDetails(
	ctx context.Context,
	table *Table,
	getCols func(context.Context) ([]Column, *errors.XError),
	getIdxs func(context.Context) ([]Index, *errors.XError),
	getFKs func(context.Context) ([]ForeignKey, *errors.XError),
) (*Table, *errors.XError) {
	var (
		columns []Column
		indexes []Index
		fks     []ForeignKey
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		res, xe := getCols(gctx)
		if xe != nil {
			return xe
		}
		columns = res
		return nil
	})
	g.Go(func() error {
		res, xe := getIdxs(gctx)
		if xe != nil {
			return xe
		}
		indexes = res
		return nil
	})
	g.Go(func() error {
		res, xe := getFKs(gctx)
		if xe != nil {
			return xe
		}
		fks = res
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, errors.AsOrWrap(err)
	}

	table.Columns = columns
	table.Indexes = indexes
	table.ForeignKeys = fks
	return table, nil
}

// ScanForeignKeyRows scans ordered foreign key rows into sorted ForeignKey structs.
func ScanForeignKeyRows(rows *sql.Rows) ([]ForeignKey, *errors.XError) {
	fkMap := make(map[string]*ForeignKey)
	for rows.Next() {
		var constraintName, columnName, refTable, refColumn string
		var ordinalPosition int
		if err := rows.Scan(&constraintName, &columnName, &refTable, &refColumn, &ordinalPosition); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan foreign key row", nil, err)
		}

		if fk, exists := fkMap[constraintName]; exists {
			fk.Columns = append(fk.Columns, columnName)
			fk.ReferencedColumns = append(fk.ReferencedColumns, refColumn)
		} else {
			fkMap[constraintName] = &ForeignKey{
				Name:              constraintName,
				Columns:           []string{columnName},
				ReferencedTable:   refTable,
				ReferencedColumns: []string{refColumn},
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}

	names := make([]string, 0, len(fkMap))
	for name := range fkMap {
		names = append(names, name)
	}
	sort.Strings(names)

	fks := make([]ForeignKey, 0, len(names))
	for _, name := range names {
		fks = append(fks, *fkMap[name])
	}
	return fks, nil
}

// ToLikePattern converts wildcard patterns (* and ?) into SQL LIKE patterns while escaping literal \, %, and _.
func ToLikePattern(pattern string) string {
	var b strings.Builder
	b.Grow(len(pattern))
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\', '%', '_':
			b.WriteByte('\\')
			b.WriteByte(pattern[i])
		case '*':
			b.WriteByte('%')
		case '?':
			b.WriteByte('_')
		default:
			b.WriteByte(pattern[i])
		}
	}
	return b.String()
}

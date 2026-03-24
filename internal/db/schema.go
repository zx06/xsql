package db

import (
	"context"
	"database/sql"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

// SchemaInfo represents database schema information.
type SchemaInfo struct {
	Database string  `json:"database" yaml:"database"`
	Tables   []Table `json:"tables" yaml:"tables"`
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

// SchemaDriver is the schema export interface.
// A Driver may optionally implement this interface to support schema export.
type SchemaDriver interface {
	Driver
	// DumpSchema exports the database schema.
	DumpSchema(ctx context.Context, db *sql.DB, opts SchemaOptions) (*SchemaInfo, *errors.XError)
}

// DumpSchema exports the database schema.
// It checks whether the driver implements the SchemaDriver interface.
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

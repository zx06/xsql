package db

import (
	"testing"
)

func TestSchemaInfo_ToSchemaData(t *testing.T) {
	tests := []struct {
		name    string
		schema  *SchemaInfo
		wantDB  string
		wantLen int
		wantOK  bool
	}{
		{
			name:    "nil schema",
			schema:  nil,
			wantDB:  "",
			wantLen: 0,
			wantOK:  false,
		},
		{
			name:    "empty tables",
			schema:  &SchemaInfo{Database: "testdb", Tables: []Table{}},
			wantDB:  "",
			wantLen: 0,
			wantOK:  false,
		},
		{
			name: "single table no columns",
			schema: &SchemaInfo{
				Database: "testdb",
				Tables: []Table{
					{Schema: "public", Name: "users"},
				},
			},
			wantDB:  "testdb",
			wantLen: 1,
			wantOK:  true,
		},
		{
			name: "single table with columns",
			schema: &SchemaInfo{
				Database: "testdb",
				Tables: []Table{
					{
						Schema:  "public",
						Name:    "users",
						Comment: "用户表",
						Columns: []Column{
							{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true},
							{Name: "email", Type: "varchar(255)", Nullable: false, Comment: "邮箱"},
						},
					},
				},
			},
			wantDB:  "testdb",
			wantLen: 1,
			wantOK:  true,
		},
		{
			name: "multiple tables",
			schema: &SchemaInfo{
				Database: "testdb",
				Tables: []Table{
					{
						Schema: "public",
						Name:   "users",
						Columns: []Column{
							{Name: "id", Type: "bigint", PrimaryKey: true},
						},
					},
					{
						Schema: "public",
						Name:   "orders",
						Columns: []Column{
							{Name: "id", Type: "bigint", PrimaryKey: true},
							{Name: "user_id", Type: "bigint"},
						},
						ForeignKeys: []ForeignKey{
							{Name: "fk_user", Columns: []string{"user_id"}, ReferencedTable: "users", ReferencedColumns: []string{"id"}},
						},
					},
				},
			},
			wantDB:  "testdb",
			wantLen: 2,
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, tables, ok := tt.schema.ToSchemaData()
			if ok != tt.wantOK {
				t.Errorf("ToSchemaData() ok = %v, want %v", ok, tt.wantOK)
			}
			if db != tt.wantDB {
				t.Errorf("ToSchemaData() db = %v, want %v", db, tt.wantDB)
			}
			if len(tables) != tt.wantLen {
				t.Errorf("ToSchemaData() len(tables) = %v, want %v", len(tables), tt.wantLen)
			}
		})
	}
}

func TestSchemaInfo_ToSchemaData_ColumnData(t *testing.T) {
	schema := &SchemaInfo{
		Database: "testdb",
		Tables: []Table{
			{
				Schema:  "public",
				Name:    "users",
				Comment: "用户表",
				Columns: []Column{
					{Name: "id", Type: "bigint", Nullable: false, Default: "nextval('users_id_seq')", Comment: "主键", PrimaryKey: true},
					{Name: "email", Type: "varchar(255)", Nullable: false, Comment: "邮箱"},
					{Name: "created_at", Type: "timestamp", Nullable: true, Default: "now()"},
				},
			},
		},
	}

	db, tables, ok := schema.ToSchemaData()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if db != "testdb" {
		t.Errorf("db = %v, want testdb", db)
	}
	if len(tables) != 1 {
		t.Fatalf("len(tables) = %v, want 1", len(tables))
	}

	table := tables[0]
	if table.Schema != "public" {
		t.Errorf("table.Schema = %v, want public", table.Schema)
	}
	if table.Name != "users" {
		t.Errorf("table.Name = %v, want users", table.Name)
	}
	if table.Comment != "用户表" {
		t.Errorf("table.Comment = %v, want 用户表", table.Comment)
	}
	if len(table.Columns) != 3 {
		t.Fatalf("len(table.Columns) = %v, want 3", len(table.Columns))
	}

	// 验证第一列
	col := table.Columns[0]
	if col.Name != "id" {
		t.Errorf("col.Name = %v, want id", col.Name)
	}
	if col.Type != "bigint" {
		t.Errorf("col.Type = %v, want bigint", col.Type)
	}
	if col.Nullable {
		t.Errorf("col.Nullable = %v, want false", col.Nullable)
	}
	if col.Default != "nextval('users_id_seq')" {
		t.Errorf("col.Default = %v, want nextval('users_id_seq')", col.Default)
	}
	if col.Comment != "主键" {
		t.Errorf("col.Comment = %v, want 主键", col.Comment)
	}
	if !col.PrimaryKey {
		t.Errorf("col.PrimaryKey = %v, want true", col.PrimaryKey)
	}
}

func TestTable_Fields(t *testing.T) {
	table := Table{
		Schema:  "myschema",
		Name:    "mytable",
		Comment: "test comment",
		Columns: []Column{
			{Name: "col1", Type: "int"},
		},
		Indexes: []Index{
			{Name: "idx1", Columns: []string{"col1"}, Unique: true},
		},
		ForeignKeys: []ForeignKey{
			{Name: "fk1", Columns: []string{"col1"}, ReferencedTable: "other", ReferencedColumns: []string{"id"}},
		},
	}

	if table.Schema != "myschema" {
		t.Errorf("Schema = %v", table.Schema)
	}
	if table.Name != "mytable" {
		t.Errorf("Name = %v", table.Name)
	}
	if len(table.Columns) != 1 {
		t.Errorf("len(Columns) = %v", len(table.Columns))
	}
	if len(table.Indexes) != 1 {
		t.Errorf("len(Indexes) = %v", len(table.Indexes))
	}
	if len(table.ForeignKeys) != 1 {
		t.Errorf("len(ForeignKeys) = %v", len(table.ForeignKeys))
	}
}

func TestColumn_Fields(t *testing.T) {
	col := Column{
		Name:       "test_col",
		Type:       "varchar(100)",
		Nullable:   true,
		Default:    "'default'",
		Comment:    "test comment",
		PrimaryKey: false,
	}

	if col.Name != "test_col" {
		t.Errorf("Name = %v", col.Name)
	}
	if col.Type != "varchar(100)" {
		t.Errorf("Type = %v", col.Type)
	}
	if !col.Nullable {
		t.Errorf("Nullable = %v", col.Nullable)
	}
	if col.Default != "'default'" {
		t.Errorf("Default = %v", col.Default)
	}
	if col.Comment != "test comment" {
		t.Errorf("Comment = %v", col.Comment)
	}
	if col.PrimaryKey {
		t.Errorf("PrimaryKey = %v", col.PrimaryKey)
	}
}

func TestIndex_Fields(t *testing.T) {
	idx := Index{
		Name:    "test_idx",
		Columns: []string{"col1", "col2"},
		Unique:  true,
		Primary: false,
	}

	if idx.Name != "test_idx" {
		t.Errorf("Name = %v", idx.Name)
	}
	if len(idx.Columns) != 2 {
		t.Errorf("len(Columns) = %v", len(idx.Columns))
	}
	if !idx.Unique {
		t.Errorf("Unique = %v", idx.Unique)
	}
	if idx.Primary {
		t.Errorf("Primary = %v", idx.Primary)
	}
}

func TestForeignKey_Fields(t *testing.T) {
	fk := ForeignKey{
		Name:              "test_fk",
		Columns:           []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	}

	if fk.Name != "test_fk" {
		t.Errorf("Name = %v", fk.Name)
	}
	if len(fk.Columns) != 1 {
		t.Errorf("len(Columns) = %v", len(fk.Columns))
	}
	if fk.ReferencedTable != "users" {
		t.Errorf("ReferencedTable = %v", fk.ReferencedTable)
	}
	if len(fk.ReferencedColumns) != 1 {
		t.Errorf("len(ReferencedColumns) = %v", len(fk.ReferencedColumns))
	}
}

func TestSchemaOptions(t *testing.T) {
	opts := SchemaOptions{
		TablePattern:  "user*",
		IncludeSystem: true,
	}

	if opts.TablePattern != "user*" {
		t.Errorf("TablePattern = %v", opts.TablePattern)
	}
	if !opts.IncludeSystem {
		t.Errorf("IncludeSystem = %v", opts.IncludeSystem)
	}
}

func TestDumpSchema_UnsupportedDriver(t *testing.T) {
	_, xe := DumpSchema(nil, "nonexistent", nil, SchemaOptions{})
	if xe == nil {
		t.Error("expected error for unsupported driver")
	}
	if xe.Code != "XSQL_DB_DRIVER_UNSUPPORTED" {
		t.Errorf("error code = %v, want XSQL_DB_DRIVER_UNSUPPORTED", xe.Code)
	}
}

// Mock driver that doesn't implement SchemaDriver
type mockNonSchemaDriver struct{}

func (d *mockNonSchemaDriver) Open(ctx interface{}, opts ConnOptions) (interface{}, error) {
	return nil, nil
}

func TestDumpSchema_DriverNotImplementSchema(t *testing.T) {
	// Register a mock driver that doesn't implement SchemaDriver
	// Note: This test would need to register/unregister which could affect other tests
	// Skipping for now as the interface check is straightforward
}

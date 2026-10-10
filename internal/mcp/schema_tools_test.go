package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/stats"
)

func TestSchemaListTables_MissingProfile(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaListTables(context.Background(), &mcp.CallToolRequest{}, SchemaListTablesInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", msgProfileRequired)
}

func TestSchemaDump_MissingProfile(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaDump(context.Background(), &mcp.CallToolRequest{}, SchemaDumpInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", msgProfileRequired)
}

func TestSchemaDescribeTable_MissingProfile(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaDescribeTable(context.Background(), &mcp.CallToolRequest{}, SchemaDescribeTableInput{
		Table: "users",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", msgProfileRequired)
}

func TestSchemaDescribeTable_MissingTable(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {DB: "mysql"},
		},
	}
	handler := NewToolHandler(cfg, stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaDescribeTable(context.Background(), &mcp.CallToolRequest{}, SchemaDescribeTableInput{
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", "table is required")
}

func TestSchemaDescribeTable_ProfileNotFound(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaDescribeTable(context.Background(), &mcp.CallToolRequest{}, SchemaDescribeTableInput{
		Profile: "missing",
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", "")
}

func TestRegisterTools_IncludesSchemaTools(t *testing.T) {
	cfg := &config.File{
		Profiles: map[string]config.Profile{"dev": {DB: "mysql"}},
	}
	handler := NewToolHandler(cfg, stats.StatsConfig{}, ServerOptions{})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	handler.RegisterTools(server)
}

func TestSchemaListTablesHandler_InvalidJSON(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: []byte("{")},
	}
	result, err := handler.schemaListTablesHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", msgInvalidInput)
}

func TestSchemaDumpHandler_InvalidJSON(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: []byte("not-json")},
	}
	result, err := handler.schemaDumpHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", msgInvalidInput)
}

func TestSchemaDescribeTableHandler_InvalidJSON(t *testing.T) {
	handler := NewToolHandler(emptyMCPConfig(), stats.StatsConfig{}, ServerOptions{})
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: []byte("{")},
	}
	result, err := handler.schemaDescribeTableHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", msgInvalidInput)
}

func TestSchemaListTables_SuccessAndRequestOptions(t *testing.T) {
	handler := testSchemaHandler(t, "mysql")
	var captured app.TableListRequest
	restore := swapSchemaHooks(t, schemaHooks{
		listTables: func(_ context.Context, req app.TableListRequest) (*db.TableList, *errors.XError) {
			captured = req
			return &db.TableList{
				Database: "appdb",
				Tables: []db.TableSummary{
					{Schema: "appdb", Name: "users"},
				},
			}, nil
		},
	})
	defer restore()

	result, _, err := handler.SchemaListTables(context.Background(), &mcp.CallToolRequest{}, SchemaListTablesInput{
		Profile:       "dev",
		TablePattern:  "user*",
		IncludeSystem: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, result, func(data map[string]any) {
		if data["database"] != "appdb" {
			t.Fatalf("database: %v", data["database"])
		}
	})
	if captured.TablePattern != "user*" || !captured.IncludeSystem {
		t.Fatalf("unexpected list request: %+v", captured)
	}
	if captured.Profile.DB != "mysql" {
		t.Fatalf("profile not passed: %+v", captured.Profile)
	}
}

func TestSchemaListTables_AppError(t *testing.T) {
	handler := testSchemaHandler(t, "mysql")
	restore := swapSchemaHooks(t, schemaHooks{
		listTables: func(context.Context, app.TableListRequest) (*db.TableList, *errors.XError) {
			return nil, errors.New(errors.CodeDBConnectFailed, "connect failed", nil)
		},
	})
	defer restore()

	result, _, err := handler.SchemaListTables(context.Background(), &mcp.CallToolRequest{}, SchemaListTablesInput{
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "DB_CONNECT_FAILED", "")
}

func TestSchemaDump_SuccessWithPattern(t *testing.T) {
	handler := testSchemaHandler(t, "pg")
	var captured app.SchemaDumpRequest
	restore := swapSchemaHooks(t, schemaHooks{
		dumpSchema: func(_ context.Context, req app.SchemaDumpRequest) (*db.SchemaInfo, *errors.XError) {
			captured = req
			return &db.SchemaInfo{
				Database: "proddb",
				Tables: []db.Table{
					{Schema: "public", Name: "orders", Columns: []db.Column{{Name: "id", Type: "bigint"}}},
				},
			}, nil
		},
	})
	defer restore()

	result, _, err := handler.SchemaDump(context.Background(), &mcp.CallToolRequest{}, SchemaDumpInput{
		Profile:      "dev",
		TablePattern: "order*",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, result, func(data map[string]any) {
		tables, ok := data["tables"].([]any)
		if !ok || len(tables) != 1 {
			t.Fatalf("tables: %v", data["tables"])
		}
	})
	if captured.TablePattern != "order*" {
		t.Fatalf("pattern not forwarded: %+v", captured)
	}
}

func TestSchemaDump_AppError(t *testing.T) {
	handler := testSchemaHandler(t, "mysql")
	restore := swapSchemaHooks(t, schemaHooks{
		dumpSchema: func(context.Context, app.SchemaDumpRequest) (*db.SchemaInfo, *errors.XError) {
			return nil, errors.New(errors.CodeInternal, "dump failed", nil)
		},
	})
	defer restore()

	result, _, err := handler.SchemaDump(context.Background(), &mcp.CallToolRequest{}, SchemaDumpInput{Profile: "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "INTERNAL", "")
}

func TestSchemaDescribeTable_SuccessWithPGSchema(t *testing.T) {
	handler := testSchemaHandler(t, "pg")
	var captured app.TableDescribeRequest
	restore := swapSchemaHooks(t, schemaHooks{
		describeTable: func(_ context.Context, req app.TableDescribeRequest) (*db.Table, *errors.XError) {
			captured = req
			return &db.Table{
				Schema:  "public",
				Name:    "users",
				Columns: []db.Column{{Name: "id", Type: "integer", PrimaryKey: true}},
			}, nil
		},
	})
	defer restore()

	result, _, err := handler.SchemaDescribeTable(context.Background(), &mcp.CallToolRequest{}, SchemaDescribeTableInput{
		Profile: "dev",
		Schema:  "public",
		Table:   "users",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolSuccess(t, result, func(data map[string]any) {
		if data["name"] != "users" {
			t.Fatalf("name: %v", data["name"])
		}
	})
	if captured.Schema != "public" || captured.Name != "users" {
		t.Fatalf("describe request: %+v", captured)
	}
}

func TestSchemaDescribeTable_TableNotFound(t *testing.T) {
	handler := testSchemaHandler(t, "pg")
	restore := swapSchemaHooks(t, schemaHooks{
		describeTable: func(context.Context, app.TableDescribeRequest) (*db.Table, *errors.XError) {
			return nil, errors.New(errors.CodeCfgInvalid, "table not found", map[string]any{"reason": "table_not_found"})
		},
	})
	defer restore()

	result, _, err := handler.SchemaDescribeTable(context.Background(), &mcp.CallToolRequest{}, SchemaDescribeTableInput{
		Profile: "dev",
		Schema:  "public",
		Table:   "missing_table",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertToolError(t, result, "CFG_INVALID", "table not found")
}

func TestSchemaListTablesHandler_Success(t *testing.T) {
	handler := testSchemaHandler(t, "mysql")
	restore := swapSchemaHooks(t, schemaHooks{
		listTables: func(context.Context, app.TableListRequest) (*db.TableList, *errors.XError) {
			return &db.TableList{Database: "d"}, nil
		},
	})
	defer restore()

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: []byte(`{"profile":"dev"}`)},
	}
	result, err := handler.schemaListTablesHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success from handler")
	}
}

func TestSchemaDumpHandler_Success(t *testing.T) {
	handler := testSchemaHandler(t, "mysql")
	restore := swapSchemaHooks(t, schemaHooks{
		dumpSchema: func(context.Context, app.SchemaDumpRequest) (*db.SchemaInfo, *errors.XError) {
			return &db.SchemaInfo{Database: "d"}, nil
		},
	})
	defer restore()

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: []byte(`{"profile":"dev"}`)},
	}
	result, err := handler.schemaDumpHandler(context.Background(), req)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("expected success, err=%v result=%+v", err, result)
	}
}

func TestSchemaDescribeTableHandler_Success(t *testing.T) {
	handler := testSchemaHandler(t, "pg")
	restore := swapSchemaHooks(t, schemaHooks{
		describeTable: func(context.Context, app.TableDescribeRequest) (*db.Table, *errors.XError) {
			return &db.Table{Name: "t"}, nil
		},
	})
	defer restore()

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: []byte(`{"profile":"dev","table":"t","schema":"public"}`)},
	}
	result, err := handler.schemaDescribeTableHandler(context.Background(), req)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("expected success, err=%v result=%+v", err, result)
	}
}

func TestSchemaListTables_RecordsStats(t *testing.T) {
	handler := testSchemaHandler(t, "mysql")
	handler.stats = stats.StatsConfig{Enabled: true, FilePath: t.TempDir() + "/stats.jsonl"}
	restore := swapSchemaHooks(t, schemaHooks{
		listTables: func(context.Context, app.TableListRequest) (*db.TableList, *errors.XError) {
			return &db.TableList{Database: "d", Tables: nil}, nil
		},
	})
	defer restore()

	_, _, err := handler.SchemaListTables(context.Background(), &mcp.CallToolRequest{}, SchemaListTablesInput{Profile: "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func emptyMCPConfig() *config.File {
	return &config.File{Profiles: map[string]config.Profile{}}
}

func testSchemaHandler(t *testing.T, dbType string) *ToolHandler {
	t.Helper()
	cfg := &config.File{
		Profiles: map[string]config.Profile{
			"dev": {DB: dbType, Database: "appdb"},
		},
	}
	return NewToolHandler(cfg, stats.StatsConfig{}, ServerOptions{})
}

type schemaHooks struct {
	listTables    func(context.Context, app.TableListRequest) (*db.TableList, *errors.XError)
	dumpSchema    func(context.Context, app.SchemaDumpRequest) (*db.SchemaInfo, *errors.XError)
	describeTable func(context.Context, app.TableDescribeRequest) (*db.Table, *errors.XError)
}

func swapSchemaHooks(t *testing.T, h schemaHooks) func() {
	t.Helper()
	prevList := schemaApp.ListTables
	prevDump := schemaApp.DumpSchema
	prevDescribe := schemaApp.DescribeTable
	if h.listTables != nil {
		schemaApp.ListTables = h.listTables
	}
	if h.dumpSchema != nil {
		schemaApp.DumpSchema = h.dumpSchema
	}
	if h.describeTable != nil {
		schemaApp.DescribeTable = h.describeTable
	}
	return func() {
		schemaApp.ListTables = prevList
		schemaApp.DumpSchema = prevDump
		schemaApp.DescribeTable = prevDescribe
	}
}

func assertToolError(t *testing.T, result *mcp.CallToolResult, code, messageContains string) {
	t.Helper()
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, code) {
		t.Fatalf("expected code %s in %s", code, text)
	}
	if messageContains != "" && !strings.Contains(text, messageContains) {
		t.Fatalf("expected message %q in %s", messageContains, text)
	}
	var envelope struct {
		OK             bool `json:"ok"`
		SchemaVersion  int  `json:"schema_version"`
		Error          map[string]any `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("invalid error json: %v", err)
	}
	if envelope.OK || envelope.SchemaVersion != 1 || envelope.Error == nil {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestSchemaListTables_EmptyDatabaseTablesArray(t *testing.T) {
	prev := schemaApp.ListTables
	t.Cleanup(func() { schemaApp.ListTables = prev })
	schemaApp.ListTables = func(context.Context, app.TableListRequest) (*db.TableList, *errors.XError) {
		return &db.TableList{Database: "empty", Tables: []db.TableSummary{}}, nil
	}
	cfg := &config.File{
		Profiles: map[string]config.Profile{"dev": {DB: "mysql"}},
	}
	handler := NewToolHandler(cfg, stats.StatsConfig{}, ServerOptions{})
	result, _, err := handler.SchemaListTables(context.Background(), &mcp.CallToolRequest{}, SchemaListTablesInput{Profile: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	assertToolSuccess(t, result, func(data map[string]any) {
		raw, ok := data["tables"].([]any)
		if !ok {
			t.Fatalf("tables should be array, got %T", data["tables"])
		}
		if raw == nil || len(raw) != 0 {
			t.Fatalf("expected empty tables array, got %v", data["tables"])
		}
	})
}

func assertToolSuccess(t *testing.T, result *mcp.CallToolResult, check func(map[string]any)) {
	t.Helper()
	if result == nil || result.IsError {
		t.Fatalf("expected success, got %+v", result)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	var envelope struct {
		OK            bool           `json:"ok"`
		SchemaVersion int            `json:"schema_version"`
		Data          map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("invalid success json: %v", err)
	}
	if !envelope.OK || envelope.SchemaVersion != 1 || envelope.Data == nil {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if check != nil {
		check(envelope.Data)
	}
}

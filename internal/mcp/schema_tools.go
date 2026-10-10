package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/errors"
)

const defaultMCPSchemaTimeout = 60 * time.Second

// SchemaListTablesInput is input for schema_list_tables.
type SchemaListTablesInput struct {
	Profile       string `json:"profile" jsonschema:"Profile name to use"`
	TablePattern  string `json:"table_pattern,omitempty" jsonschema:"Table name filter (* and ? wildcards)"`
	IncludeSystem bool   `json:"include_system,omitempty" jsonschema:"Include system tables"`
}

// SchemaDumpInput is input for schema_dump.
type SchemaDumpInput struct {
	Profile       string `json:"profile" jsonschema:"Profile name to use"`
	TablePattern  string `json:"table_pattern,omitempty" jsonschema:"Table name filter (* and ? wildcards)"`
	IncludeSystem bool   `json:"include_system,omitempty" jsonschema:"Include system tables"`
}

// SchemaDescribeTableInput is input for schema_describe_table.
type SchemaDescribeTableInput struct {
	Profile string `json:"profile" jsonschema:"Profile name to use"`
	Schema  string `json:"schema,omitempty" jsonschema:"Schema name (PostgreSQL; optional for MySQL)"`
	Table   string `json:"table" jsonschema:"Table name"`
}

func (h *ToolHandler) registerSchemaTools(server *mcp.Server, profileEnums []any) {
	profileProp := &jsonschema.Schema{
		Type:        "string",
		Description: "Profile name to use",
		Enum:        profileEnums,
	}
	optionalTablePattern := &jsonschema.Schema{
		Type:        "string",
		Description: "Table name filter (supports * and ? wildcards)",
	}
	optionalIncludeSystem := &jsonschema.Schema{
		Type:        "boolean",
		Description: "Include system tables",
	}

	server.AddTool(&mcp.Tool{
		Name:        "schema_list_tables",
		Description: "List database tables (lightweight summary for schema discovery)",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"profile"},
			Properties: map[string]*jsonschema.Schema{
				"profile":        profileProp,
				"table_pattern":  optionalTablePattern,
				"include_system": optionalIncludeSystem,
			},
		},
	}, h.schemaListTablesHandler)

	server.AddTool(&mcp.Tool{
		Name:        "schema_dump",
		Description: "Dump database schema (tables, columns, indexes, foreign keys); use table_pattern to limit size",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"profile"},
			Properties: map[string]*jsonschema.Schema{
				"profile":        profileProp,
				"table_pattern":  optionalTablePattern,
				"include_system": optionalIncludeSystem,
			},
		},
	}, h.schemaDumpHandler)

	server.AddTool(&mcp.Tool{
		Name:        "schema_describe_table",
		Description: "Describe a single table (columns, indexes, foreign keys)",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"profile", "table"},
			Properties: map[string]*jsonschema.Schema{
				"profile": profileProp,
				"schema":  {Type: "string", Description: "Schema name (required for PostgreSQL when not public)"},
				"table":   {Type: "string", Description: "Table name"},
			},
		},
	}, h.schemaDescribeTableHandler)
}

func (h *ToolHandler) schemaListTablesHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input SchemaListTablesInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return h.errorFromErr(errors.Wrap(errors.CodeCfgInvalid, "invalid input", nil, err)), nil
	}
	result, _, err := h.SchemaListTables(ctx, req, input)
	return result, err
}

func (h *ToolHandler) schemaDumpHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input SchemaDumpInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return h.errorFromErr(errors.Wrap(errors.CodeCfgInvalid, "invalid input", nil, err)), nil
	}
	result, _, err := h.SchemaDump(ctx, req, input)
	return result, err
}

func (h *ToolHandler) schemaDescribeTableHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input SchemaDescribeTableInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return h.errorFromErr(errors.Wrap(errors.CodeCfgInvalid, "invalid input", nil, err)), nil
	}
	result, _, err := h.SchemaDescribeTable(ctx, req, input)
	return result, err
}

// SchemaListTables lists tables via internal/app.
func (h *ToolHandler) SchemaListTables(ctx context.Context, req *mcp.CallToolRequest, input SchemaListTablesInput) (*mcp.CallToolResult, any, error) {
	if input.Profile == "" {
		return h.errorResult(errors.New(errors.CodeCfgInvalid, "profile is required", nil)), nil, nil
	}
	profile, xe := h.resolveProfile(input.Profile)
	if xe != nil {
		return h.errorResult(xe), nil, nil
	}

	timeout := app.SchemaTimeout(profile, 0, false, defaultMCPSchemaTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, xe := app.ListTables(ctx, app.TableListRequest{
		Profile:          profile,
		TablePattern:     input.TablePattern,
		IncludeSystem:    input.IncludeSystem,
		AllowPlaintext:   profile.AllowPlaintext,
		SkipHostKeyCheck: false,
	})
	h.recordMCPStats("schema_list_tables", input.Profile, xe == nil, time.Since(start), xe, "")

	if xe != nil {
		return h.errorResult(xe), nil, nil
	}
	return h.successResult(result)
}

// SchemaDump dumps schema via internal/app.
func (h *ToolHandler) SchemaDump(ctx context.Context, req *mcp.CallToolRequest, input SchemaDumpInput) (*mcp.CallToolResult, any, error) {
	if input.Profile == "" {
		return h.errorResult(errors.New(errors.CodeCfgInvalid, "profile is required", nil)), nil, nil
	}
	profile, xe := h.resolveProfile(input.Profile)
	if xe != nil {
		return h.errorResult(xe), nil, nil
	}

	timeout := app.SchemaTimeout(profile, 0, false, defaultMCPSchemaTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, xe := app.DumpSchema(ctx, app.SchemaDumpRequest{
		Profile:          profile,
		TablePattern:     input.TablePattern,
		IncludeSystem:    input.IncludeSystem,
		AllowPlaintext:   profile.AllowPlaintext,
		SkipHostKeyCheck: false,
	})
	h.recordMCPStats("schema_dump", input.Profile, xe == nil, time.Since(start), xe, "")

	if xe != nil {
		return h.errorResult(xe), nil, nil
	}
	return h.successResult(result)
}

// SchemaDescribeTable describes one table via internal/app.
func (h *ToolHandler) SchemaDescribeTable(ctx context.Context, req *mcp.CallToolRequest, input SchemaDescribeTableInput) (*mcp.CallToolResult, any, error) {
	if input.Profile == "" {
		return h.errorResult(errors.New(errors.CodeCfgInvalid, "profile is required", nil)), nil, nil
	}
	if input.Table == "" {
		return h.errorResult(errors.New(errors.CodeCfgInvalid, "table is required", nil)), nil, nil
	}
	profile, xe := h.resolveProfile(input.Profile)
	if xe != nil {
		return h.errorResult(xe), nil, nil
	}

	timeout := app.SchemaTimeout(profile, 0, false, defaultMCPSchemaTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, xe := app.DescribeTable(ctx, app.TableDescribeRequest{
		Profile:          profile,
		Schema:           input.Schema,
		Name:             input.Table,
		AllowPlaintext:   profile.AllowPlaintext,
		SkipHostKeyCheck: false,
	})
	h.recordMCPStats("schema_describe_table", input.Profile, xe == nil, time.Since(start), xe, "")

	if xe != nil {
		return h.errorResult(xe), nil, nil
	}
	return h.successResult(result)
}

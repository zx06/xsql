package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/stats"
)

const defaultMCPQueryTimeout = 30 * time.Second

// QueryInput represents the input for the query tool
type QueryInput struct {
	SQL     string `json:"sql" jsonschema:"SQL query to execute"`
	Profile string `json:"profile" jsonschema:"Profile name to use"`
	MaxRows *int   `json:"max_rows,omitempty" jsonschema:"Maximum rows to return (positive integer; MCP default 500)"`
}

// ProfileShowInput represents the input for the profile_show tool
type ProfileShowInput struct {
	Name string `json:"name" jsonschema:"Profile name"`
}

// ServerOptions configures MCP server behavior passed from the CLI entrypoint.
type ServerOptions struct {
	UnsafeAllowWrite bool // server-level write opt-in (dual auth with profile)
}

// ToolHandler manages MCP tools
type ToolHandler struct {
	config            *config.File
	stats             stats.StatsConfig
	serverAllowsWrite bool
}

// NewToolHandler creates a new tool handler
func NewToolHandler(cfg *config.File, st stats.StatsConfig, opts ServerOptions) *ToolHandler {
	if cfg == nil {
		cfg = &config.File{
			Profiles:   map[string]config.Profile{},
			SSHProxies: map[string]config.SSHProxy{},
		}
	}
	return &ToolHandler{
		config:            cfg,
		stats:             st,
		serverAllowsWrite: opts.UnsafeAllowWrite,
	}
}

// getProfileNames returns a list of available profile names
func (h *ToolHandler) getProfileNames() []string {
	names := make([]string, 0, len(h.config.Profiles))
	for name := range h.config.Profiles {
		names = append(names, name)
	}
	return names
}

func (h *ToolHandler) resolveProfile(name string) (config.Profile, *errors.XError) {
	return app.ResolveProfile(*h.config, name)
}

// RegisterTools registers all tools with the MCP server
func (h *ToolHandler) RegisterTools(server *mcp.Server) {
	profileNames := h.getProfileNames()
	profileEnums := make([]any, len(profileNames))
	for i, name := range profileNames {
		profileEnums[i] = name
	}

	// Query tool with profile enum
	querySchema := &jsonschema.Schema{
		Type:     "object",
		Required: []string{"sql", "profile"},
		Properties: map[string]*jsonschema.Schema{
			"sql": {
				Type:        "string",
				Description: "SQL query to execute",
			},
			"profile": {
				Type:        "string",
				Description: "Profile name to use",
				Enum:        profileEnums,
			},
			"max_rows": {
				Type:        "integer",
				Description: "Maximum rows to return (positive integer; default from profile/mcp config or 500)",
			},
		},
	}
	server.AddTool(&mcp.Tool{
		Name:        "query",
		Description: "Execute SQL query on database",
		InputSchema: querySchema,
	}, h.queryHandler)

	// Profile list tool
	mcp.AddTool[struct{}, any](server, &mcp.Tool{
		Name:        "profile_list",
		Description: "List all configured profiles",
	}, h.ProfileList)

	// Profile show tool with profile enum
	profileShowSchema := &jsonschema.Schema{
		Type:     "object",
		Required: []string{"name"},
		Properties: map[string]*jsonschema.Schema{
			"name": {
				Type:        "string",
				Description: "Profile name",
				Enum:        profileEnums,
			},
		},
	}
	server.AddTool(&mcp.Tool{
		Name:        "profile_show",
		Description: "Show profile details",
		InputSchema: profileShowSchema,
	}, h.profileShowHandler)

	h.registerSchemaTools(server, profileEnums)
}

// queryHandler is the raw handler for query tool
func (h *ToolHandler) queryHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input QueryInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return h.errorFromErr(errors.Wrap(errors.CodeCfgInvalid, "invalid input", nil, err)), nil
	}
	result, _, err := h.Query(ctx, req, input)
	return result, err
}

// profileShowHandler is the raw handler for profile_show tool
func (h *ToolHandler) profileShowHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input ProfileShowInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return h.errorFromErr(errors.Wrap(errors.CodeCfgInvalid, "invalid input", nil, err)), nil
	}
	result, _, err := h.ProfileShow(ctx, req, input)
	return result, err
}

// Query executes a SQL query via internal/app.
func (h *ToolHandler) Query(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) {
	if input.SQL == "" {
		return h.errorResult(errors.New(errors.CodeCfgInvalid, "sql is required", nil)), nil, nil
	}
	if input.Profile == "" {
		return h.errorResult(errors.New(errors.CodeCfgInvalid, "profile is required", nil)), nil, nil
	}

	profile, xe := h.resolveProfile(input.Profile)
	if xe != nil {
		return h.errorResult(xe), nil, nil
	}

	argSet := input.MaxRows != nil
	argVal := 0
	if argSet {
		argVal = *input.MaxRows
	}
	maxRows, xe := app.ResolveMCPQueryMaxRows(profile, h.config.MCP, argVal, argSet)
	if xe != nil {
		return h.errorResult(xe), nil, nil
	}

	timeout := app.QueryTimeout(profile, 0, false, defaultMCPQueryTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, xe := app.Query(ctx, app.QueryRequest{
		Profile:          profile,
		SQL:              input.SQL,
		AllowPlaintext:   profile.AllowPlaintext,
		SkipHostKeyCheck: false,
		UnsafeAllowWrite: WriteAllowed(h.serverAllowsWrite, profile.UnsafeAllowWrite),
		MaxRows:          maxRows,
		TruncationHint:   app.QueryTruncationHint,
	})
	h.recordMCPStats("query", input.Profile, xe == nil, time.Since(start), xe, input.SQL)

	if xe != nil {
		return h.errorResult(xe), nil, nil
	}
	result = applyMCPResponseByteLimit(result, app.ResolveMCPMaxResponseBytes(h.config.MCP))
	return h.successResult(result)
}

// ProfileList lists all profiles
func (h *ToolHandler) ProfileList(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
	profiles := make([]config.ProfileInfo, 0, len(h.config.Profiles))
	for name, p := range h.config.Profiles {
		profiles = append(profiles, config.ProfileToInfo(name, p))
	}
	return h.successResult(map[string]any{"profiles": profiles})
}

// ProfileShow shows profile details
func (h *ToolHandler) ProfileShow(ctx context.Context, req *mcp.CallToolRequest, input ProfileShowInput) (*mcp.CallToolResult, any, error) {
	profile, xe := h.resolveProfile(input.Name)
	if xe != nil {
		return h.errorResult(xe), nil, nil
	}
	p := h.config.Profiles[input.Name]

	result := map[string]any{
		"name":               input.Name,
		"description":        p.Description,
		"db":                 profile.DB,
		"host":               profile.Host,
		"port":               profile.Port,
		"user":               profile.User,
		"database":           profile.Database,
		"unsafe_allow_write": profile.UnsafeAllowWrite,
		"allow_plaintext":    profile.AllowPlaintext,
	}
	if p.DSN != "" {
		result["dsn"] = "***"
	}
	if p.Password != "" {
		result["password"] = "***"
	}
	if profile.SSHProxy != "" {
		result["ssh_proxy"] = profile.SSHProxy
		if profile.SSHConfig != nil {
			result["ssh_host"] = profile.SSHConfig.Host
			result["ssh_port"] = profile.SSHConfig.Port
			result["ssh_user"] = profile.SSHConfig.User
			if profile.SSHConfig.IdentityFile != "" {
				result["ssh_identity_file"] = profile.SSHConfig.IdentityFile
			}
		}
	}
	return h.successResult(result)
}

// formatError formats an error as JSON
func (h *ToolHandler) formatError(err error) string {
	var xe *errors.XError
	if err != nil {
		xe = errors.AsOrWrap(err)
	} else {
		xe = errors.New(errors.CodeInternal, "unknown error", nil)
	}
	output := map[string]any{
		"ok":             false,
		"schema_version": 1,
		"error": map[string]any{
			"code":    xe.Code,
			"message": xe.Message,
			"details": xe.Details,
		},
	}
	jsonData, jsonErr := json.MarshalIndent(output, "", "  ")
	if jsonErr != nil {
		return `{"ok":false,"error":{"code":"XSQL_INTERNAL","message":"failed to format error"}}`
	}
	return string(jsonData)
}

// CreateServer creates a new MCP server
func CreateServer(version string, cfg *config.File, st stats.StatsConfig, opts ServerOptions) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "xsql",
		Version: version,
	}, nil)

	handler := NewToolHandler(cfg, st, opts)
	handler.RegisterTools(server)

	return server, nil
}

// recordMCPStats records an MCP query to the stats store.
func (h *ToolHandler) recordMCPStats(cmd, profile string, ok bool, duration time.Duration, xe *errors.XError, sql string) {
	if !h.stats.Enabled {
		return
	}
	store := stats.NewStore(h.stats.FilePath)
	r := &stats.Record{
		Timestamp:  time.Now(),
		Cmd:        cmd,
		Profile:    profile,
		OK:         ok,
		DurationMs: duration.Milliseconds(),
	}
	if xe != nil {
		r.ErrorCode = string(xe.Code)
	}
	if h.stats.LogSQL && sql != "" {
		r.SQL = sql
	}
	_ = store.Append(r)
}

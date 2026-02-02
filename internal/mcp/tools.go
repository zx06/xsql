package mcp

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/ssh"
)

// QueryInput represents the input for the query tool
type QueryInput struct {
	SQL     string `json:"sql" jsonschema:"SQL query to execute"`
	Profile string `json:"profile" jsonschema:"Profile name to use"`
}

// ProfileShowInput represents the input for the profile_show tool
type ProfileShowInput struct {
	Name string `json:"name" jsonschema:"Profile name"`
}

// ToolHandler manages MCP tools
type ToolHandler struct {
	config *config.File
}

// NewToolHandler creates a new tool handler
func NewToolHandler(cfg *config.File) *ToolHandler {
	return &ToolHandler{
		config: cfg,
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
}

// queryHandler is the raw handler for query tool
func (h *ToolHandler) queryHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input QueryInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.Wrap(errors.CodeCfgInvalid, "invalid input", nil, err))},
			},
		}, nil
	}
	result, _, err := h.Query(ctx, req, input)
	return result, err
}

// profileShowHandler is the raw handler for profile_show tool
func (h *ToolHandler) profileShowHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input ProfileShowInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.Wrap(errors.CodeCfgInvalid, "invalid input", nil, err))},
			},
		}, nil
	}
	result, _, err := h.ProfileShow(ctx, req, input)
	return result, err
}

// Query executes a SQL query
func (h *ToolHandler) Query(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) {
	// Validate required fields
	if input.SQL == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.New(errors.CodeCfgInvalid, "sql is required", nil))},
			},
		}, nil, nil
	}

	if input.Profile == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.New(errors.CodeCfgInvalid, "profile is required", nil))},
			},
		}, nil, nil
	}

	// Get profile
	profile := h.getProfile(input.Profile)
	if profile == nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.New(errors.CodeCfgInvalid, "profile does not exist", map[string]any{"name": input.Profile, "reason": "profile_not_found"}))},
			},
		}, nil, nil
	}

	if profile.DB == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil))},
			},
		}, nil, nil
	}

	// Parse password
	password := profile.Password
	if password != "" {
		pw, xe := secret.Resolve(password, secret.Options{AllowPlaintext: profile.AllowPlaintext})
		if xe != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: h.formatError(xe)},
				},
			}, nil, nil
		}
		password = pw
	}

	// SSH proxy
	var sshClient *ssh.Client
	if profile.SSHConfig != nil {
		passphrase := profile.SSHConfig.Passphrase
		if passphrase != "" {
			pp, xe := secret.Resolve(passphrase, secret.Options{AllowPlaintext: profile.AllowPlaintext})
			if xe != nil {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{
						&mcp.TextContent{Text: h.formatError(xe)},
					},
				}, nil, nil
			}
			passphrase = pp
		}
		sshOpts := ssh.Options{
			Host:           profile.SSHConfig.Host,
			Port:           profile.SSHConfig.Port,
			User:           profile.SSHConfig.User,
			IdentityFile:   profile.SSHConfig.IdentityFile,
			Passphrase:     passphrase,
			KnownHostsFile: profile.SSHConfig.KnownHostsFile,
		}
		sc, xe := ssh.Connect(ctx, sshOpts)
		if xe != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: h.formatError(xe)},
				},
			}, nil, nil
		}
		defer sc.Close()
		sshClient = sc
	}

	// Get driver
	drv, ok := db.Get(profile.DB)
	if !ok {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.New(errors.CodeDBDriverUnsupported, "unsupported db driver", map[string]any{"db": profile.DB}))},
			},
		}, nil, nil
	}

	connOpts := db.ConnOptions{
		DSN:      profile.DSN,
		Host:     profile.Host,
		Port:     profile.Port,
		User:     profile.User,
		Password: password,
		Database: profile.Database,
	}
	if sshClient != nil {
		connOpts.Dialer = sshClient
	}

	conn, xe := drv.Open(ctx, connOpts)
	if xe != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(xe)},
			},
		}, nil, nil
	}
	defer conn.Close()

	// Query options - use read-only mode by default
	result, xe := db.Query(ctx, conn, input.SQL, db.QueryOptions{
		UnsafeAllowWrite: profile.UnsafeAllowWrite,
		DBType:           profile.DB,
	})
	if xe != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(xe)},
			},
		}, nil, nil
	}

	output := map[string]any{
		"ok":             true,
		"schema_version": 1,
		"data":           result,
	}
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.Wrap(errors.CodeInternal, "failed to marshal result", nil, err))},
			},
		}, nil, nil
	}

	// Return result directly in content per RFC
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}, nil, nil
}

// ProfileList lists all profiles
func (h *ToolHandler) ProfileList(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
	type profileInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		DB          string `json:"db"`
		Mode        string `json:"mode"`
	}

	profiles := make([]profileInfo, 0, len(h.config.Profiles))
	for name, p := range h.config.Profiles {
		mode := "read-only"
		if p.UnsafeAllowWrite {
			mode = "read-write"
		}
		profiles = append(profiles, profileInfo{
			Name:        name,
			Description: p.Description,
			DB:          p.DB,
			Mode:        mode,
		})
	}

	output := map[string]any{
		"ok":             true,
		"schema_version": 1,
		"data": map[string]any{
			"profiles": profiles,
		},
	}
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.Wrap(errors.CodeInternal, "failed to marshal result", nil, err))},
			},
		}, nil, nil
	}

	// Return result directly in content per RFC
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}, nil, nil
}

// ProfileShow shows profile details
func (h *ToolHandler) ProfileShow(ctx context.Context, req *mcp.CallToolRequest, input ProfileShowInput) (*mcp.CallToolResult, any, error) {
	profile, ok := h.config.Profiles[input.Name]
	if !ok {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.New(errors.CodeCfgInvalid, "profile does not exist", map[string]any{"name": input.Name, "reason": "profile_not_found"}))},
			},
		}, nil, nil
	}

	// Redact sensitive information
	result := map[string]any{
		"name":               input.Name,
		"description":        profile.Description,
		"db":                 profile.DB,
		"host":               profile.Host,
		"port":               profile.Port,
		"user":               profile.User,
		"database":           profile.Database,
		"unsafe_allow_write": profile.UnsafeAllowWrite,
		"allow_plaintext":    profile.AllowPlaintext,
	}
	if profile.DSN != "" {
		result["dsn"] = "***"
	}
	if profile.Password != "" {
		result["password"] = "***"
	}
	if profile.SSHProxy != "" {
		result["ssh_proxy"] = profile.SSHProxy
		if proxy, ok := h.config.SSHProxies[profile.SSHProxy]; ok {
			result["ssh_host"] = proxy.Host
			result["ssh_port"] = proxy.Port
			result["ssh_user"] = proxy.User
			if proxy.IdentityFile != "" {
				result["ssh_identity_file"] = proxy.IdentityFile
			}
		}
	}

	output := map[string]any{
		"ok":             true,
		"schema_version": 1,
		"data":           result,
	}
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: h.formatError(errors.Wrap(errors.CodeInternal, "failed to marshal result", nil, err))},
			},
		}, nil, nil
	}

	// Return result directly in content per RFC
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}, nil, nil
}

// getProfile gets a profile by name, or returns the default profile
func (h *ToolHandler) getProfile(name string) *config.Profile {
	if name == "" {
		// Use default profile (first one)
		for _, p := range h.config.Profiles {
			return &p
		}
		return nil
	}

	profile, ok := h.config.Profiles[name]
	if !ok {
		return nil
	}
	return &profile
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
	jsonData, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonData)
}

// CreateServer creates a new MCP server
func CreateServer(version string, cfg *config.File) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "xsql",
		Version: version,
	}, nil)

	handler := NewToolHandler(cfg)
	handler.RegisterTools(server)

	return server, nil
}

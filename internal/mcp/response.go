package mcp

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/errors"
)

func (h *ToolHandler) successResult(data any) (*mcp.CallToolResult, any, error) {
	jsonData, err := json.MarshalIndent(map[string]any{
		"ok":             true,
		"schema_version": 1,
		"data":           data,
	}, "", "  ")
	if err != nil {
		return h.errorResult(errors.Wrap(errors.CodeInternal, "failed to marshal result", nil, err)), nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}, nil, nil
}

func (h *ToolHandler) errorResult(xe *errors.XError) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: h.formatError(xe)},
		},
	}
}

func (h *ToolHandler) errorFromErr(err error) *mcp.CallToolResult {
	if err == nil {
		return h.errorResult(errors.New(errors.CodeInternal, "unknown error", nil))
	}
	return h.errorResult(errors.AsOrWrap(err))
}

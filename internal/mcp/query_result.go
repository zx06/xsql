package mcp

import (
	"encoding/json"
	"strings"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/db"
)

func applyMCPResponseByteLimit(result *db.QueryResult, maxBytes int) *db.QueryResult {
	if result == nil || maxBytes <= 0 {
		return result
	}
	byteHint := app.MCPResponseTruncationHint
	for {
		payload, err := json.Marshal(map[string]any{
			"ok":             true,
			"schema_version": 1,
			"data":           result,
		})
		if err != nil || len(payload) <= maxBytes || len(result.Rows) == 0 {
			return result
		}
		result.Rows = result.Rows[:len(result.Rows)-1]
		result.Truncated = true
		result.RowCount = len(result.Rows)
		if result.MaxRows == 0 {
			result.MaxRows = result.RowCount
		}
		if result.Hint == "" {
			result.Hint = app.QueryTruncationHint
		} else if !strings.Contains(result.Hint, byteHint) {
			result.Hint = result.Hint + " " + byteHint
		}
	}
}

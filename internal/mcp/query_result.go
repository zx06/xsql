package mcp

import (
	"encoding/json"

	"github.com/zx06/xsql/internal/db"
)

func applyMCPResponseByteLimit(result *db.QueryResult, maxBytes int, effectiveMaxRows int) *db.QueryResult {
	if result == nil || maxBytes <= 0 {
		return result
	}
	if effectiveMaxRows > 0 && result.MaxRows == 0 {
		result.MaxRows = effectiveMaxRows
	}
	if len(result.Rows) == 0 {
		return result
	}
	if mcpPayloadSize(result) <= maxBytes {
		return result
	}

	keep := binarySearchRowFit(result, maxBytes)
	if keep >= len(result.Rows) {
		return result
	}

	result.Rows = result.Rows[:keep]
	result.Truncated = true
	result.RowCount = keep
	result.MaxRows = effectiveMaxRows
	result.MaxResponseBytes = maxBytes
	result.TruncatedReason = db.AppendTruncatedReason(result.TruncatedReason, db.TruncatedReasonMaxResponseBytes)
	db.RefreshTruncationHint(result)
	return result
}

func mcpPayloadSize(result *db.QueryResult) int {
	payload, err := json.Marshal(map[string]any{
		"ok":             true,
		"schema_version": 1,
		"data":           result,
	})
	if err != nil {
		return 0
	}
	return len(payload)
}

func binarySearchRowFit(result *db.QueryResult, maxBytes int) int {
	low, high := 0, len(result.Rows)
	for low < high {
		mid := (low + high + 1) / 2
		trial := &db.QueryResult{
			Columns:             result.Columns,
			Rows:                result.Rows[:mid],
			Truncated:           result.Truncated,
			RowCount:            mid,
			MaxRows:             result.MaxRows,
			MaxRowsRequested:    result.MaxRowsRequested,
			MaxRowsLimitApplied: result.MaxRowsLimitApplied,
			TruncatedReason:     result.TruncatedReason,
			MaxResponseBytes:    maxBytes,
			Hint:                result.Hint,
		}
		if mcpPayloadSize(trial) <= maxBytes {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return low
}

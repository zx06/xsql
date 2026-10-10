package mcp

import (
	"encoding/json"
	"testing"

	"github.com/zx06/xsql/internal/db"
)

func TestApplyMCPResponseByteLimit(t *testing.T) {
	rows := make([]map[string]any, 0, 50)
	for i := 0; i < 50; i++ {
		rows = append(rows, map[string]any{"payload": string(make([]byte, 200))})
	}
	result := &db.QueryResult{
		Columns: []string{"payload"},
		Rows:    rows,
	}
	out := applyMCPResponseByteLimit(result, 2048)
	payload, _ := json.Marshal(map[string]any{"ok": true, "schema_version": 1, "data": out})
	if len(payload) > 2048 {
		t.Fatalf("expected payload under 2048 bytes, got %d", len(payload))
	}
	if !out.Truncated {
		t.Fatal("expected truncated")
	}
}

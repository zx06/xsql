package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

func TestApplyMCPResponseByteLimit_Noop(t *testing.T) {
	result := &db.QueryResult{Columns: []string{"n"}, Rows: []map[string]any{{"n": 1}}}
	out := applyMCPResponseByteLimit(result, 0, 500)
	if out != result || out.Truncated {
		t.Fatalf("expected noop, got %+v", out)
	}
}

func TestApplyMCPResponseByteLimit(t *testing.T) {
	rows := make([]map[string]any, 0, 50)
	for i := 0; i < 50; i++ {
		rows = append(rows, map[string]any{"payload": string(make([]byte, 200))})
	}
	result := &db.QueryResult{
		Columns: []string{"payload"},
		Rows:    rows,
		MaxRows: 500,
	}
	out := applyMCPResponseByteLimit(result, 2048, 500)
	payload, _ := json.Marshal(map[string]any{"ok": true, "schema_version": 1, "data": out})
	if len(payload) > 2048 {
		t.Fatalf("expected payload under 2048 bytes, got %d", len(payload))
	}
	if !out.Truncated {
		t.Fatal("expected truncated")
	}
	if out.MaxRows != 500 {
		t.Fatalf("max_rows=%d want configured 500", out.MaxRows)
	}
	if !strings.Contains(out.TruncatedReason, db.TruncatedReasonMaxResponseBytes) {
		t.Fatalf("reason=%q", out.TruncatedReason)
	}
	if out.MaxResponseBytes != 2048 {
		t.Fatalf("max_response_bytes=%d", out.MaxResponseBytes)
	}
	if !strings.Contains(out.Hint, errors.HintMCPResponseTruncated) {
		t.Fatalf("hint=%q", out.Hint)
	}
}

func TestApplyMCPResponseByteLimit_BothReasons(t *testing.T) {
	rows := make([]map[string]any, 0, 20)
	for i := 0; i < 20; i++ {
		rows = append(rows, map[string]any{"payload": string(make([]byte, 100))})
	}
	result := &db.QueryResult{
		Columns:         []string{"payload"},
		Rows:            rows,
		Truncated:       true,
		TruncatedReason: db.TruncatedReasonMaxRows,
		MaxRows:         20,
		Hint:            errors.HintQueryResultTruncated,
	}
	out := applyMCPResponseByteLimit(result, 1024, 20)
	if !strings.Contains(out.TruncatedReason, db.TruncatedReasonMaxRows) ||
		!strings.Contains(out.TruncatedReason, db.TruncatedReasonMaxResponseBytes) {
		t.Fatalf("reason=%q", out.TruncatedReason)
	}
	if !strings.Contains(out.Hint, errors.HintQueryResultTruncated) ||
		!strings.Contains(out.Hint, errors.HintMCPResponseTruncated) {
		t.Fatalf("hint=%q", out.Hint)
	}
}

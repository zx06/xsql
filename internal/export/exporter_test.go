package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/db"
)

func TestExportQueryResult_CSV_JSON_MD(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "xsql_export_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	res := &db.QueryResult{
		Columns: []string{"id", "username", "status"},
		Rows: []map[string]any{
			{"id": 1, "username": "alice", "status": "active"},
			{"id": 2, "username": "bob", "status": nil},
		},
	}

	// 1. CSV
	csvPath := filepath.Join(tempDir, "test.csv")
	absPath, xe := ExportQueryResult(res, FormatCSV, csvPath)
	if xe != nil {
		t.Fatalf("CSV export failed: %v", xe)
	}
	content, _ := os.ReadFile(absPath)
	if !strings.Contains(string(content), "username") || !strings.Contains(string(content), "alice") {
		t.Fatalf("unexpected CSV content: %s", string(content))
	}

	// 2. JSON
	jsonPath := filepath.Join(tempDir, "test.json")
	absPath, xe = ExportQueryResult(res, FormatJSON, jsonPath)
	if xe != nil {
		t.Fatalf("JSON export failed: %v", xe)
	}
	content, _ = os.ReadFile(absPath)
	if !strings.Contains(string(content), `"alice"`) {
		t.Fatalf("unexpected JSON content: %s", string(content))
	}

	// 3. Markdown
	mdPath := filepath.Join(tempDir, "sub", "test.md")
	res.Rows[1]["status"] = "multiline\ntext|pipe"
	absPath, xe = ExportQueryResult(res, FormatMarkdown, mdPath)
	if xe != nil {
		t.Fatalf("Markdown export failed: %v", xe)
	}
	content, _ = os.ReadFile(absPath)
	if !strings.Contains(string(content), "| username |") || !strings.Contains(string(content), "text\\|pipe") {
		t.Fatalf("unexpected Markdown content: %s", string(content))
	}

	// 4. Nil result & empty filePath fallback
	_, xe = ExportQueryResult(nil, FormatCSV, "")
	if xe == nil {
		t.Fatal("expected error for nil QueryResult")
	}

	absDefault, xe := ExportQueryResult(res, FormatCSV, "")
	if xe != nil {
		t.Fatalf("unexpected error for empty filePath: %v", xe)
	}
	_ = os.Remove(absDefault)

	// 5. Unsupported formats must fail before creating or truncating a file.
	invalidPath := filepath.Join(tempDir, "invalid.xlsx")
	if _, xe = ExportQueryResult(res, ExportFormat("xlsx"), invalidPath); xe == nil {
		t.Fatal("expected unsupported export format error")
	}
	if _, err := os.Stat(invalidPath); !os.IsNotExist(err) {
		t.Fatalf("unsupported format should not create a file, stat err=%v", err)
	}
}

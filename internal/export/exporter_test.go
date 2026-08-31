package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/db"
)

func TestExportQueryResult_CSV_JSON(t *testing.T) {
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

	// 3. Markdown is no longer supported in export_data
	mdPath := filepath.Join(tempDir, "test.md")
	if _, xe = ExportQueryResult(res, ExportFormat("markdown"), mdPath); xe == nil {
		t.Fatal("expected error for markdown format in ExportQueryResult")
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

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	pEmpty, err := ExpandPath("")
	if err != nil || pEmpty != "" {
		t.Fatalf("expected empty, got %q, err %v", pEmpty, err)
	}

	pHomeOnly, err := ExpandPath("~")
	if err != nil || pHomeOnly != home {
		t.Fatalf("expected %s, got %s", home, pHomeOnly)
	}

	p, err := ExpandPath("~/Downloads/report.md")
	if err != nil {
		t.Fatalf("ExpandPath failed: %v", err)
	}
	expected := filepath.Join(home, "Downloads/report.md")
	if p != expected {
		t.Fatalf("expected %s, got %s", expected, p)
	}

	pWin, err := ExpandPath("~\\Downloads\\report.md")
	if err != nil {
		t.Fatalf("ExpandPath failed: %v", err)
	}
	expectedWin := filepath.Join(home, "Downloads\\report.md")
	if pWin != expectedWin {
		t.Fatalf("expected %s, got %s", expectedWin, pWin)
	}

	p2, _ := ExpandPath("/absolute/path.txt")
	if p2 != "/absolute/path.txt" {
		t.Fatalf("expected /absolute/path.txt, got %s", p2)
	}
}

func TestExportReport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "xsql_report_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reportPath := filepath.Join(tempDir, "sub", "summary.md")
	content := "# Daily Report\n\n- Total users: 100\n"
	absPath, xe := ExportReport(content, reportPath)
	if xe != nil {
		t.Fatalf("ExportReport failed: %v", xe)
	}

	readBack, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}
	if string(readBack) != content {
		t.Fatalf("unexpected content: %s", string(readBack))
	}

	// Test default report name when path is empty
	absDef, xe := ExportReport("# Test", "")
	if xe != nil {
		t.Fatalf("ExportReport with empty path failed: %v", xe)
	}
	_ = os.Remove(absDef)
}

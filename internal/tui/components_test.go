package tui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/zx06/xsql/internal/db"
)

func TestSanitizeCellWithCJK(t *testing.T) {
	// Test Chinese wide character display width calculation & truncation
	cjkStr := "机器下架"
	if runewidth.StringWidth(cjkStr) != 8 {
		t.Fatalf("expected runewidth 8 for '机器下架', got %d", runewidth.StringWidth(cjkStr))
	}

	sanitized, wasTruncated := sanitizeCellWithStatus(cjkStr, 6)
	if !wasTruncated {
		t.Fatal("expected wasTruncated to be true")
	}
	if strings.Contains(sanitized, "\n") {
		t.Fatal("sanitized string must never contain newlines")
	}
}

func TestFormatTableResult_CJKBorderProtection(t *testing.T) {
	res := &db.QueryResult{
		Columns: []string{"id", "status"},
		Rows: []map[string]any{
			{"id": "1", "status": "故障"},
			{"id": "2", "status": "机器下架"},
			{"id": "3", "status": "正常运行"},
		},
	}

	formatted := FormatTableResult(res, 0, 0, 80, true)
	lines := strings.Split(formatted, "\n")

	// Verify that rows are strictly single line per data row
	for i, line := range lines {
		if strings.Contains(line, "故障") {
			if !strings.Contains(line, "1") {
				t.Errorf("line %d split '故障' into new line: %q", i, line)
			}
		}
	}

	// Nil and empty result fallbacks
	if nilOut := FormatTableResult(nil, 0, 0, 80, false); !strings.Contains(nilOut, "empty dataset") {
		t.Errorf("expected empty dataset message for nil QueryResult, got %q", nilOut)
	}

	if nilVert := FormatVerticalResult(nil); !strings.Contains(nilVert, "empty dataset") {
		t.Errorf("expected empty dataset message for nil QueryResult vertical view, got %q", nilVert)
	}

	emptyRes := &db.QueryResult{Columns: []string{"id"}, Rows: []map[string]any{}}
	if emptyOut := FormatTableResult(emptyRes, 0, 0, 80, false); !strings.Contains(emptyOut, "0 rows returned") {
		t.Fatalf("expected '0 rows returned', got %q", emptyOut)
	}

	// Test offset and inactive view
	offsetFormatted := FormatTableResult(res, 1, 1, 40, false)
	if !strings.Contains(offsetFormatted, "status") {
		t.Errorf("expected status column in offset view, got:\n%s", offsetFormatted)
	}

	// Test sanitizeCell & sanitizeCellWithStatus
	if s, _ := sanitizeCellWithStatus(nil, 10); s != "NULL" {
		t.Errorf("expected 'NULL' for nil input, got %q", s)
	}

	if s := sanitizeCell("short", 10); s != "short" {
		t.Errorf("expected 'short', got %q", s)
	}
}

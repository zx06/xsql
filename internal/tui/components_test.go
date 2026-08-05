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
}

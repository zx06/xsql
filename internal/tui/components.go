package tui

import (
	"fmt"
	"strings"

	"github.com/zx06/xsql/internal/db"
)

func FormatTableResult(result *db.QueryResult) string {
	if result == nil || len(result.Columns) == 0 {
		return "(No data returned)"
	}

	var sb strings.Builder
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}

	for _, row := range result.Rows {
		for i, col := range result.Columns {
			val := fmt.Sprintf("%v", row[col])
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}

	// Print Headers
	var headerRow []string
	var lineRow []string
	for i, col := range result.Columns {
		headerRow = append(headerRow, fmt.Sprintf("%-*s", widths[i], col))
		lineRow = append(lineRow, strings.Repeat("-", widths[i]))
	}
	sb.WriteString(strings.Join(headerRow, "  ") + "\n")
	sb.WriteString(strings.Join(lineRow, "  ") + "\n")

	// Print Rows (up to 50 rows)
	maxRows := len(result.Rows)
	if maxRows > 50 {
		maxRows = 50
	}
	for i := 0; i < maxRows; i++ {
		var rowValues []string
		for j, col := range result.Columns {
			val := fmt.Sprintf("%v", result.Rows[i][col])
			rowValues = append(rowValues, fmt.Sprintf("%-*s", widths[j], val))
		}
		sb.WriteString(strings.Join(rowValues, "  ") + "\n")
	}

	if len(result.Rows) > 50 {
		sb.WriteString(fmt.Sprintf("\n... and %d more rows\n", len(result.Rows)-50))
	}

	return sb.String()
}

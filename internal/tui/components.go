package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/zx06/xsql/internal/db"
)

var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7D56F4")).
				Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C0CAF5")).
			Padding(0, 1)

	TableNilStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565F89")).
			Italic(true).
			Padding(0, 1)

	TableBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B4261"))
)

// FormatTableResult renders a beautiful terminal box table for SQL query results.
func FormatTableResult(result *db.QueryResult) string {
	if result == nil || len(result.Columns) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(No columns or empty dataset returned)")
	}

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(0 rows returned)")
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(TableBorderStyle).
		Headers(result.Columns...)

	// Configure header styling
	t.StyleFunc(func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return TableHeaderStyle
		}
		return TableCellStyle
	})

	// Add data rows (limit to 50 rows for viewport cleanliness)
	maxRows := len(result.Rows)
	if maxRows > 50 {
		maxRows = 50
	}

	for i := 0; i < maxRows; i++ {
		var rowValues []string
		for _, col := range result.Columns {
			val := result.Rows[i][col]
			if val == nil {
				rowValues = append(rowValues, TableNilStyle.Render("NULL"))
			} else {
				valStr := fmt.Sprintf("%v", val)
				// Clean formatting for time string or long text
				rowValues = append(rowValues, valStr)
			}
		}
		t.Row(rowValues...)
	}

	var sb strings.Builder
	sb.WriteString(t.Render())

	if len(result.Rows) > 50 {
		moreStr := fmt.Sprintf("\n... and %d more rows (truncated for performance)", len(result.Rows)-50)
		sb.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render(moreStr))
	}

	return sb.String()
}

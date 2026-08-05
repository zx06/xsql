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

	FieldKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AA2F7"))

	FieldValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C0CAF5"))

	RecordDividerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(PrimaryColor)
)

const MaxColumnWidth = 28 // Max character width per cell to prevent border alignment breakage

// FormatTableResult renders a beautiful terminal box table for SQL query results.
func FormatTableResult(result *db.QueryResult) string {
	if result == nil || len(result.Columns) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(No columns or empty dataset returned)")
	}

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(0 rows returned)")
	}

	// Dynamic column width allocation
	maxCellWidth := MaxColumnWidth
	if len(result.Columns) > 10 {
		maxCellWidth = 20
	} else if len(result.Columns) > 15 {
		maxCellWidth = 15
	}

	// Truncate and clean column headers
	headers := make([]string, len(result.Columns))
	for i, col := range result.Columns {
		headers[i] = sanitizeCell(col, maxCellWidth)
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(TableBorderStyle).
		Headers(headers...)

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

	hasTruncatedCell := false

	for i := 0; i < maxRows; i++ {
		var rowValues []string
		for _, col := range result.Columns {
			val := result.Rows[i][col]
			if val == nil {
				rowValues = append(rowValues, TableNilStyle.Render("NULL"))
			} else {
				cellStr, wasTruncated := sanitizeCellWithStatus(val, maxCellWidth)
				if wasTruncated {
					hasTruncatedCell = true
				}
				rowValues = append(rowValues, cellStr)
			}
		}
		t.Row(rowValues...)
	}

	var sb strings.Builder
	sb.WriteString(t.Render())

	var footerNotes []string
	if len(result.Rows) > 50 {
		footerNotes = append(footerNotes, fmt.Sprintf("... and %d more rows", len(result.Rows)-50))
	}
	if hasTruncatedCell {
		footerNotes = append(footerNotes, "long text truncated; press Ctrl+V for Full Vertical View")
	}

	if len(footerNotes) > 0 {
		noteStr := "\n(" + strings.Join(footerNotes, " | ") + ")"
		sb.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render(noteStr))
	}

	return sb.String()
}

// FormatVerticalResult renders SQL query results in full vertical (psql \x expanded) format with NO truncation.
func FormatVerticalResult(result *db.QueryResult) string {
	if result == nil || len(result.Columns) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(No columns or empty dataset returned)")
	}

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(0 rows returned)")
	}

	maxKeyLen := 0
	for _, col := range result.Columns {
		if len(col) > maxKeyLen {
			maxKeyLen = len(col)
		}
	}

	maxRows := len(result.Rows)
	if maxRows > 50 {
		maxRows = 50
	}

	var sb strings.Builder
	for i := 0; i < maxRows; i++ {
		divider := fmt.Sprintf("━ Record %d of %d ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", i+1, len(result.Rows))
		sb.WriteString(RecordDividerStyle.Render(divider) + "\n")

		for _, col := range result.Columns {
			val := result.Rows[i][col]
			keyStr := FieldKeyStyle.Render(fmt.Sprintf("%-*s", maxKeyLen, col))
			
			if val == nil {
				sb.WriteString(fmt.Sprintf("  %s : %s\n", keyStr, TableNilStyle.Render("NULL")))
			} else {
				valStr := fmt.Sprintf("%v", val)
				// Full display with indentation for multiline text
				if strings.Contains(valStr, "\n") {
					indented := strings.ReplaceAll(valStr, "\n", "\n    ")
					sb.WriteString(fmt.Sprintf("  %s :\n    %s\n", keyStr, FieldValueStyle.Render(indented)))
				} else {
					sb.WriteString(fmt.Sprintf("  %s : %s\n", keyStr, FieldValueStyle.Render(valStr)))
				}
			}
		}
		sb.WriteString("\n")
	}

	if len(result.Rows) > 50 {
		sb.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render(fmt.Sprintf("... and %d more rows truncated\n", len(result.Rows)-50)))
	}

	return sb.String()
}

func sanitizeCell(val any, maxLen int) string {
	res, _ := sanitizeCellWithStatus(val, maxLen)
	return res
}

func sanitizeCellWithStatus(val any, maxLen int) (string, bool) {
	if val == nil {
		return "NULL", false
	}
	s := fmt.Sprintf("%v", val)
	// Replace all line breaks with spaces so the box border line never breaks
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)

	runes := []rune(s)
	if len(runes) > maxLen {
		if maxLen <= 3 {
			return string(runes[:maxLen]), true
		}
		return string(runes[:maxLen-3]) + "...", true
	}
	return s, false
}

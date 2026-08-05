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

	ScrollBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF75B5"))
)

const MaxColumnWidth = 24 // Max character width per cell to keep grid compact

// FormatTableResult renders a beautiful terminal box table for SQL query results with horizontal column scrolling.
func FormatTableResult(result *db.QueryResult, colOffset int, termWidth int) string {
	if result == nil || len(result.Columns) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(No columns or empty dataset returned)")
	}

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(0 rows returned)")
	}

	if termWidth <= 20 {
		termWidth = 80
	}

	// Calculate maximum width needed for each column
	totalCols := len(result.Columns)
	if colOffset >= totalCols {
		colOffset = totalCols - 1
	}
	if colOffset < 0 {
		colOffset = 0
	}

	colWidths := make([]int, totalCols)
	for i, col := range result.Columns {
		w := len(col)
		if w > MaxColumnWidth {
			w = MaxColumnWidth
		}
		// Inspect sample rows for width calculation
		sampleCount := len(result.Rows)
		if sampleCount > 20 {
			sampleCount = 20
		}
		for r := 0; r < sampleCount; r++ {
			val := result.Rows[r][col]
			if val != nil {
				cellStr := fmt.Sprintf("%v", val)
				cellStr = strings.ReplaceAll(cellStr, "\n", " ")
				runesLen := len([]rune(cellStr))
				if runesLen > w {
					w = runesLen
				}
			}
		}
		if w > MaxColumnWidth {
			w = MaxColumnWidth
		}
		if w < 6 {
			w = 6
		}
		// Add padding (2 chars) + border (1 char)
		colWidths[i] = w + 3
	}

	// Determine visible column range [startCol, endCol) that fits within termWidth - 6
	availWidth := termWidth - 8
	if availWidth < 30 {
		availWidth = 30
	}

	startCol := colOffset
	endCol := startCol
	accumWidth := 0

	for i := startCol; i < totalCols; i++ {
		if accumWidth+colWidths[i] > availWidth && endCol > startCol {
			break
		}
		accumWidth += colWidths[i]
		endCol = i + 1
	}

	visibleCols := result.Columns[startCol:endCol]

	// Truncate and clean visible headers
	headers := make([]string, len(visibleCols))
	for i, col := range visibleCols {
		colIdx := startCol + i
		headerText := sanitizeCell(col, colWidths[colIdx]-3)
		if i == 0 && startCol > 0 {
			headerText = "◀ " + headerText
		}
		if i == len(visibleCols)-1 && endCol < totalCols {
			headerText = headerText + fmt.Sprintf(" ▶(+%d)", totalCols-endCol)
		}
		headers[i] = headerText
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

	// Add data rows (limit to 12 rows for optimal viewport visibility)
	maxRows := len(result.Rows)
	if maxRows > 12 {
		maxRows = 12
	}

	hasTruncatedCell := false

	for i := 0; i < maxRows; i++ {
		var rowValues []string
		for idx, col := range visibleCols {
			colIdx := startCol + idx
			val := result.Rows[i][col]
			if val == nil {
				rowValues = append(rowValues, TableNilStyle.Render("NULL"))
			} else {
				cellStr, wasTruncated := sanitizeCellWithStatus(val, colWidths[colIdx]-3)
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
	if len(result.Rows) > 12 {
		footerNotes = append(footerNotes, fmt.Sprintf("showing 1-12 of %d rows (Press PgUp/PgDn to scroll, Ctrl+V for Full View)", len(result.Rows)))
	}
	if startCol > 0 || endCol < totalCols {
		footerNotes = append(footerNotes, fmt.Sprintf("Showing cols %d-%d of %d (Use ←/→ keys to scroll columns)", startCol+1, endCol, totalCols))
	}
	if hasTruncatedCell {
		footerNotes = append(footerNotes, "press Ctrl+V for Full Vertical View")
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

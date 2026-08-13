package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mattn/go-runewidth"

	"github.com/zx06/xsql/internal/db"
)

var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#4F46E5", Dark: "#818CF8"}).
				Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#F1F5F9"}).
			Padding(0, 1)

	TableNilStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#94A3B8", Dark: "#64748B"}).
			Italic(true).
			Padding(0, 1)

	TableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#334155"})

	ActiveTableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#818CF8"})

	FieldKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#4F46E5", Dark: "#818CF8"})

	FieldValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#F1F5F9"})

	RecordDividerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(PrimaryColor)

	ScrollBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#BE185D", Dark: "#F472B6"})
)

const (
	MaxColumnWidth = 24 // Max character width per cell to keep grid compact
	PageRowSize    = 12 // Rows per page
)

// FormatTableResult renders a beautiful terminal box table for SQL query results with column scrolling and row pagination.
func FormatTableResult(result *db.QueryResult, colOffset int, rowOffset int, termWidth int, isActive bool) string {
	if result == nil || len(result.Columns) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(No columns or empty dataset returned)")
	}

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(0 rows returned)")
	}

	if termWidth <= 20 {
		termWidth = 80
	}

	totalRows := len(result.Rows)
	if rowOffset >= totalRows {
		rowOffset = (totalRows - 1) / PageRowSize * PageRowSize
	}
	if rowOffset < 0 {
		rowOffset = 0
	}

	// Calculate maximum display width needed for each column using runewidth for CJK support
	totalCols := len(result.Columns)
	if colOffset >= totalCols {
		colOffset = totalCols - 1
	}
	if colOffset < 0 {
		colOffset = 0
	}

	colWidths := make([]int, totalCols)
	for i, col := range result.Columns {
		w := runewidth.StringWidth(col)
		if w > MaxColumnWidth {
			w = MaxColumnWidth
		}
		// Inspect current page rows for width calculation
		endR := rowOffset + PageRowSize
		if endR > totalRows {
			endR = totalRows
		}
		for r := rowOffset; r < endR; r++ {
			val := result.Rows[r][col]
			if val != nil {
				cellStr := fmt.Sprintf("%v", val)
				cellStr = strings.ReplaceAll(cellStr, "\n", " ")
				dispLen := runewidth.StringWidth(cellStr)
				if dispLen > w {
					w = dispLen
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

	// Determine visible column range [startCol, endCol) that fits within termWidth - 8
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

	borderStyle := TableBorderStyle
	if isActive {
		borderStyle = ActiveTableBorderStyle
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		Headers(headers...)

	// Configure header styling
	t.StyleFunc(func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return TableHeaderStyle
		}
		return TableCellStyle
	})

	// Add page rows [rowOffset, min(totalRows, rowOffset+PageRowSize))
	endRow := rowOffset + PageRowSize
	if endRow > totalRows {
		endRow = totalRows
	}

	hasTruncatedCell := false

	for i := rowOffset; i < endRow; i++ {
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
	if isActive {
		footerNotes = append(footerNotes, "[FOCUSED]")
	}
	if totalRows > PageRowSize {
		footerNotes = append(footerNotes, fmt.Sprintf("rows %d-%d of %d (Press PgUp/PgDn for Page)", rowOffset+1, endRow, totalRows))
	}
	if startCol > 0 || endCol < totalCols {
		footerNotes = append(footerNotes, fmt.Sprintf("cols %d-%d of %d (Use ←/→ keys for Cols)", startCol+1, endCol, totalCols))
	}
	if hasTruncatedCell {
		footerNotes = append(footerNotes, "press Ctrl+E for Full View (Expand/Collapse)")
	}

	if len(footerNotes) > 0 {
		noteStr := "\n(" + strings.Join(footerNotes, " | ") + ")"
		if isActive {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7AA2F7")).Bold(true).Render(noteStr))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render(noteStr))
		}
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
		w := runewidth.StringWidth(col)
		if w > maxKeyLen {
			maxKeyLen = w
		}
	}

	maxRows := len(result.Rows)
	if maxRows > 500 {
		maxRows = 500
	}

	var sb strings.Builder
	for i := 0; i < maxRows; i++ {
		divider := fmt.Sprintf("━ Record %d of %d ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", i+1, len(result.Rows))
		sb.WriteString(RecordDividerStyle.Render(divider) + "\n")

		for _, col := range result.Columns {
			val := result.Rows[i][col]
			keyWidth := runewidth.StringWidth(col)
			padding := strings.Repeat(" ", max(0, maxKeyLen-keyWidth))
			keyStr := FieldKeyStyle.Render(col + padding)

			if val == nil {
				fmt.Fprintf(&sb, "  %s : %s\n", keyStr, TableNilStyle.Render("NULL"))
			} else {
				valStr := fmt.Sprintf("%v", val)
				// Full display with indentation for multiline text
				if strings.Contains(valStr, "\n") {
					indented := strings.ReplaceAll(valStr, "\n", "\n    ")
					fmt.Fprintf(&sb, "  %s :\n    %s\n", keyStr, FieldValueStyle.Render(indented))
				} else {
					fmt.Fprintf(&sb, "  %s : %s\n", keyStr, FieldValueStyle.Render(valStr))
				}
			}
		}
		sb.WriteString("\n")
	}

	if len(result.Rows) > 500 {
		sb.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render(fmt.Sprintf("... and %d more rows (truncated for performance)\n", len(result.Rows)-500)))
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

	dispLen := runewidth.StringWidth(s)
	if dispLen > maxLen {
		if maxLen <= 3 {
			return runewidth.Truncate(s, maxLen, ""), true
		}
		return runewidth.Truncate(s, maxLen, "..."), true
	}
	return s, false
}

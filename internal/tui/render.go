package tui

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/glamour"
)

// RenderMarkdown renders markdown text using Glamour with rich ANSI terminal styling.
func RenderMarkdown(md string, width int) string {
	md = strings.TrimSpace(md)
	if md == "" {
		return ""
	}
	if width <= 10 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-6),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimSpace(out)
}

// HighlightCode renders syntax-highlighted code for terminal display using Chroma.
func HighlightCode(code string, lexerName string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	var buf bytes.Buffer
	err := quick.Highlight(&buf, code, lexerName, "terminal256", "dracula")
	if err != nil {
		return code
	}
	return buf.String()
}

// HighlightSQL applies syntax highlighting to SQL statements.
func HighlightSQL(sqlStr string) string {
	return HighlightCode(sqlStr, "sql")
}

// HighlightJS applies syntax highlighting to JavaScript code blocks.
func HighlightJS(jsStr string) string {
	return HighlightCode(jsStr, "javascript")
}

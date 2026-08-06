package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	// High-Contrast Adaptive Styles for SQL & JS Syntax Highlighting (Light & Dark mode compatible)
	KeywordStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C084FC"})   // Rich Purple
	StringStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"})              // Emerald Green
	NumberStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"})              // Amber Gold
	NameStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#38BDF8"})   // Sky Blue
	CommentStyle    = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}) // Slate Grey
	DefaultTxtStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#F8FAFC"})   // Deep Slate / Crisp White
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
		glamour.WithStandardStyle("auto"),
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

// HighlightCode renders adaptive, high-contrast syntax-highlighted code for light & dark terminals.
func HighlightCode(code string, lexerName string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	lexer := lexers.Get(lexerName)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return DefaultTxtStyle.Render(code)
	}

	var sb strings.Builder
	for _, t := range iterator.Tokens() {
		val := t.Value
		switch t.Type {
		case chroma.Keyword, chroma.KeywordReserved, chroma.KeywordType, chroma.KeywordNamespace:
			sb.WriteString(KeywordStyle.Render(val))
		case chroma.String, chroma.StringChar, chroma.StringSingle, chroma.StringDouble, chroma.StringBacktick:
			sb.WriteString(StringStyle.Render(val))
		case chroma.Number, chroma.NumberInteger, chroma.NumberFloat, chroma.NumberHex, chroma.NumberOct:
			sb.WriteString(NumberStyle.Render(val))
		case chroma.Name, chroma.NameAttribute, chroma.NameClass, chroma.NameFunction, chroma.NameTag:
			sb.WriteString(NameStyle.Render(val))
		case chroma.Comment, chroma.CommentSingle, chroma.CommentMultiline:
			sb.WriteString(CommentStyle.Render(val))
		default:
			sb.WriteString(DefaultTxtStyle.Render(val))
		}
	}

	return sb.String()
}

// HighlightSQL applies adaptive high-contrast syntax highlighting to SQL statements.
func HighlightSQL(sqlStr string) string {
	return HighlightCode(sqlStr, "sql")
}

// HighlightJS applies adaptive high-contrast syntax highlighting to JavaScript code blocks.
func HighlightJS(jsStr string) string {
	return HighlightCode(jsStr, "javascript")
}

package tui

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
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

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func uintPtr(u uint) *uint {
	return &u
}

// XSQLDarkMarkdownStyle defines a refined, high-contrast Dark theme for Glamour.
// It eliminates any OSC 11 background probe (which caused garbled terminal echo in input box)
// and ensures crisp contrast for tables, headers, lists, code, and text on dark terminal backgrounds.
var XSQLDarkMarkdownStyle = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{},
		Margin:         uintPtr(0),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#94A3B8"),
		},
		Indent:      uintPtr(1),
		IndentToken: stringPtr("│ "),
	},
	List: ansi.StyleList{
		LevelIndent: 2,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Bold:        boolPtr(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           stringPtr("#FFFFFF"),
			BackgroundColor: stringPtr("#6D28D9"),
			Bold:            boolPtr(true),
		},
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "## ",
			Color:  stringPtr("#818CF8"), // Vibrant Indigo
			Bold:   boolPtr(true),
		},
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "### ",
			Color:  stringPtr("#38BDF8"), // Sky Blue
			Bold:   boolPtr(true),
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "#### ",
			Color:  stringPtr("#34D399"), // Emerald Green
			Bold:   boolPtr(true),
		},
	},
	H5: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "##### ",
			Color:  stringPtr("#F472B6"), // Pink
			Bold:   boolPtr(true),
		},
	},
	H6: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "###### ",
			Color:  stringPtr("#94A3B8"), // Slate Grey
			Bold:   boolPtr(false),
		},
	},
	Text: ansi.StylePrimitive{},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: boolPtr(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: boolPtr(true),
		Color:  stringPtr("#E2E8F0"),
	},
	Strong: ansi.StylePrimitive{
		Bold:  boolPtr(true),
		Color: stringPtr("#FFFFFF"),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  stringPtr("#475569"),
		Format: "\n────────\n",
	},
	Item: ansi.StylePrimitive{
		BlockPrefix: "• ",
		Color:       stringPtr("#818CF8"),
	},
	Enumeration: ansi.StylePrimitive{
		BlockPrefix: ". ",
		Color:       stringPtr("#818CF8"),
	},
	Task: ansi.StyleTask{
		StylePrimitive: ansi.StylePrimitive{},
		Ticked:         "[✓] ",
		Unticked:       "[ ] ",
	},
	Link: ansi.StylePrimitive{
		Color:     stringPtr("#38BDF8"),
		Underline: boolPtr(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: stringPtr("#818CF8"),
		Bold:  boolPtr(true),
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           stringPtr("#F472B6"),
			BackgroundColor: stringPtr("#1E293B"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#F8FAFC"),
			},
			Margin: uintPtr(0),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: stringPtr("#F8FAFC"),
			},
			Error: ansi.StylePrimitive{
				Color:           stringPtr("#F8FAFC"),
				BackgroundColor: stringPtr("#EF4444"),
			},
			Comment: ansi.StylePrimitive{
				Color: stringPtr("#94A3B8"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: stringPtr("#F472B6"),
			},
			Keyword: ansi.StylePrimitive{
				Color: stringPtr("#C084FC"),
				Bold:  boolPtr(true),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: stringPtr("#F472B6"),
				Bold:  boolPtr(true),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: stringPtr("#F43F5E"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: stringPtr("#A78BFA"),
			},
			Operator: ansi.StylePrimitive{
				Color: stringPtr("#38BDF8"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: stringPtr("#CBD5E1"),
			},
			Name: ansi.StylePrimitive{
				Color: stringPtr("#F8FAFC"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: stringPtr("#38BDF8"),
			},
			NameTag: ansi.StylePrimitive{
				Color: stringPtr("#F472B6"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: stringPtr("#A78BFA"),
			},
			NameClass: ansi.StylePrimitive{
				Color: stringPtr("#FBBF24"),
				Bold:  boolPtr(true),
			},
			NameConstant: ansi.StylePrimitive{
				Color: stringPtr("#F472B6"),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: stringPtr("#FBBF24"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: stringPtr("#34D399"),
				Bold:  boolPtr(true),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: stringPtr("#FBBF24"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: stringPtr("#34D399"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: stringPtr("#2DD4BF"),
			},
			GenericDeleted: ansi.StylePrimitive{
				Color: stringPtr("#F87171"),
			},
			GenericEmph: ansi.StylePrimitive{
				Italic: boolPtr(true),
			},
			GenericInserted: ansi.StylePrimitive{
				Color: stringPtr("#34D399"),
			},
			GenericStrong: ansi.StylePrimitive{
				Bold: boolPtr(true),
			},
			GenericSubheading: ansi.StylePrimitive{
				Color: stringPtr("#94A3B8"),
			},
			Background: ansi.StylePrimitive{
				BackgroundColor: stringPtr("#0F172A"),
			},
		},
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#F1F5F9"), // Table cell text is crisp white/light slate
			},
		},
		CenterSeparator: stringPtr("+"),
		ColumnSeparator: stringPtr("|"),
		RowSeparator:    stringPtr("-"),
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n🠶 ",
	},
}

// XSQLLightMarkdownStyle defines a clean, high-contrast Light theme for Glamour.
// It uses dark readable text on light backgrounds with vibrant Indigo/Sky accents.
var XSQLLightMarkdownStyle = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{},
		Margin:         uintPtr(0),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#475569"),
		},
		Indent:      uintPtr(1),
		IndentToken: stringPtr("│ "),
	},
	List: ansi.StyleList{
		LevelIndent: 2,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Bold:        boolPtr(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           stringPtr("#FFFFFF"),
			BackgroundColor: stringPtr("#4F46E5"),
			Bold:            boolPtr(true),
		},
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "## ",
			Color:  stringPtr("#4338CA"), // Deep Indigo
			Bold:   boolPtr(true),
		},
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "### ",
			Color:  stringPtr("#0284C7"), // Crisp Sky Blue
			Bold:   boolPtr(true),
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "#### ",
			Color:  stringPtr("#059669"), // Emerald Green
			Bold:   boolPtr(true),
		},
	},
	H5: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "##### ",
			Color:  stringPtr("#BE185D"), // Rose Pink
			Bold:   boolPtr(true),
		},
	},
	H6: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "###### ",
			Color:  stringPtr("#64748B"), // Slate Grey
			Bold:   boolPtr(false),
		},
	},
	Text: ansi.StylePrimitive{},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: boolPtr(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: boolPtr(true),
		Color:  stringPtr("#334155"),
	},
	Strong: ansi.StylePrimitive{
		Bold:  boolPtr(true),
		Color: stringPtr("#020617"),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  stringPtr("#CBD5E1"),
		Format: "\n────────\n",
	},
	Item: ansi.StylePrimitive{
		BlockPrefix: "• ",
		Color:       stringPtr("#4F46E5"),
	},
	Enumeration: ansi.StylePrimitive{
		BlockPrefix: ". ",
		Color:       stringPtr("#4F46E5"),
	},
	Task: ansi.StyleTask{
		StylePrimitive: ansi.StylePrimitive{},
		Ticked:         "[✓] ",
		Unticked:       "[ ] ",
	},
	Link: ansi.StylePrimitive{
		Color:     stringPtr("#0284C7"),
		Underline: boolPtr(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: stringPtr("#4338CA"),
		Bold:  boolPtr(true),
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           stringPtr("#BE185D"),
			BackgroundColor: stringPtr("#F1F5F9"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#0F172A"),
			},
			Margin: uintPtr(0),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: stringPtr("#0F172A"),
			},
			Error: ansi.StylePrimitive{
				Color:           stringPtr("#FFFFFF"),
				BackgroundColor: stringPtr("#EF4444"),
			},
			Comment: ansi.StylePrimitive{
				Color: stringPtr("#64748B"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: stringPtr("#BE185D"),
			},
			Keyword: ansi.StylePrimitive{
				Color: stringPtr("#6D28D9"),
				Bold:  boolPtr(true),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: stringPtr("#BE185D"),
				Bold:  boolPtr(true),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: stringPtr("#E11D48"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: stringPtr("#6D28D9"),
			},
			Operator: ansi.StylePrimitive{
				Color: stringPtr("#0284C7"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: stringPtr("#475569"),
			},
			Name: ansi.StylePrimitive{
				Color: stringPtr("#0F172A"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: stringPtr("#0284C7"),
			},
			NameTag: ansi.StylePrimitive{
				Color: stringPtr("#BE185D"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: stringPtr("#6D28D9"),
			},
			NameClass: ansi.StylePrimitive{
				Color: stringPtr("#B45309"),
				Bold:  boolPtr(true),
			},
			NameConstant: ansi.StylePrimitive{
				Color: stringPtr("#BE185D"),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: stringPtr("#B45309"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: stringPtr("#059669"),
				Bold:  boolPtr(true),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: stringPtr("#B45309"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: stringPtr("#059669"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: stringPtr("#0D9488"),
			},
			GenericDeleted: ansi.StylePrimitive{
				Color: stringPtr("#E11D48"),
			},
			GenericEmph: ansi.StylePrimitive{
				Italic: boolPtr(true),
			},
			GenericInserted: ansi.StylePrimitive{
				Color: stringPtr("#059669"),
			},
			GenericStrong: ansi.StylePrimitive{
				Bold: boolPtr(true),
			},
			GenericSubheading: ansi.StylePrimitive{
				Color: stringPtr("#64748B"),
			},
			Background: ansi.StylePrimitive{
				BackgroundColor: stringPtr("#F8FAFC"),
			},
		},
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#0F172A"), // Table cell text is crisp deep slate
			},
		},
		CenterSeparator: stringPtr("+"),
		ColumnSeparator: stringPtr("|"),
		RowSeparator:    stringPtr("-"),
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n🠶 ",
	},
}

// DetectDarkBackground safely determines whether the terminal has a dark background
// by inspecting COLORFGBG and environment variables, completely avoiding OSC 11 probe sequences
// which cause terminal stdin race conditions and garbled text echo in interactive TUI applications.
func DetectDarkBackground() bool {
	// 1. Explicit environment variable override
	if val := strings.ToLower(strings.TrimSpace(os.Getenv("XSQL_THEME"))); val != "" {
		if val == "light" {
			return false
		}
		if val == "dark" {
			return true
		}
	}

	// 2. Standard COLORFGBG environment variable (e.g. "15;0" -> Dark, "0;15" -> Light)
	if colorfgbg := strings.TrimSpace(os.Getenv("COLORFGBG")); colorfgbg != "" {
		parts := strings.Split(colorfgbg, ";")
		if len(parts) >= 2 {
			bgPart := strings.TrimSpace(parts[len(parts)-1])
			if bg, err := strconv.Atoi(bgPart); err == nil {
				// Standard terminal colors: 7 (light grey) and 15 (bright white) indicate a light background
				if bg == 7 || bg == 15 {
					return false
				}
				return true
			}
		}
	}

	// 3. macOS Native Appearance Detection (AppleInterfaceStyle)
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/usr/bin/defaults", "read", "-g", "AppleInterfaceStyle")
		out, err := cmd.Output()
		if err != nil {
			// On macOS, absence of AppleInterfaceStyle key indicates Light Appearance
			return false
		}
		if strings.Contains(strings.ToLower(string(out)), "dark") {
			return true
		}
		return false
	}

	// 4. Default fallback to true (Dark mode)
	return true
}

// CurrentThemeIsDark tracks the detected or configured theme preference
var CurrentThemeIsDark = DetectDarkBackground()

// SetThemeDark sets the theme preference across Lipgloss and Glamour without terminal probing
func SetThemeDark(isDark bool) {
	CurrentThemeIsDark = isDark
	lipgloss.SetHasDarkBackground(isDark)
}

// RenderMarkdown renders markdown text using Glamour with the active theme style.
func RenderMarkdown(md string, width int) string {
	return RenderMarkdownWithTheme(md, width, CurrentThemeIsDark)
}

// RenderMarkdownWithTheme renders markdown text using Glamour with an explicit theme style.
func RenderMarkdownWithTheme(md string, width int, isDark bool) string {
	md = strings.TrimSpace(md)
	if md == "" {
		return ""
	}
	if width <= 10 {
		width = 80
	}
	style := XSQLDarkMarkdownStyle
	if !isDark {
		style = XSQLLightMarkdownStyle
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
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

// ThemeChangedMsg is dispatched when the terminal/system background theme changes.
type ThemeChangedMsg struct {
	IsDark bool
}

// WatchThemeChangesCmd watches for appearance changes in the background without blocking the UI.
func WatchThemeChangesCmd(currentIsDark bool) tea.Cmd {
	return func() tea.Msg {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			newDark := DetectDarkBackground()
			if newDark != currentIsDark {
				return ThemeChangedMsg{IsDark: newDark}
			}
		}
		return nil
	}
}

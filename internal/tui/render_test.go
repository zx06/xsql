package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderMarkdown(t *testing.T) {
	out := RenderMarkdown("# Hello World\nThis is a **test**.", 80)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "Hello World") {
		t.Errorf("expected rendered markdown to contain 'Hello World', got %q (plain: %q)", out, plain)
	}
	if !strings.Contains(plain, "test") {
		t.Errorf("expected rendered markdown to contain 'test', got %q", plain)
	}

	tableMD := "| Name | Value |\n| --- | --- |\n| Total | 100 |"
	tableOut := RenderMarkdown(tableMD, 80)
	plainTable := ansi.Strip(tableOut)
	if !strings.Contains(plainTable, "Total") || !strings.Contains(plainTable, "100") {
		t.Errorf("expected table markdown to contain Total and 100, got %q", plainTable)
	}

	empty := RenderMarkdown("", 80)
	if empty != "" {
		t.Errorf("expected empty string for empty input, got %q", empty)
	}

	narrow := RenderMarkdown("# Header", 5)
	if !strings.Contains(ansi.Strip(narrow), "Header") {
		t.Errorf("expected narrow width fallback, got %q", narrow)
	}

	// Light Theme Rendering Test
	lightOut := RenderMarkdownWithTheme("# Light Title\n| A | B |\n| 1 | 2 |", 80, false)
	plainLight := ansi.Strip(lightOut)
	if !strings.Contains(plainLight, "Light Title") || !strings.Contains(plainLight, "1") {
		t.Errorf("expected light theme markdown to render correctly, got %q", plainLight)
	}

	// Global SetThemeDark Toggle Test
	SetThemeDark(false)
	outAfterLight := RenderMarkdown("## Subtitle", 80)
	if !strings.Contains(ansi.Strip(outAfterLight), "Subtitle") {
		t.Errorf("expected Subtitle in global light mode, got %q", outAfterLight)
	}

	SetThemeDark(true)
	outAfterDark := RenderMarkdown("## Dark Subtitle", 80)
	if !strings.Contains(ansi.Strip(outAfterDark), "Dark Subtitle") {
		t.Errorf("expected Dark Subtitle in global dark mode, got %q", outAfterDark)
	}
}

func TestHighlightCode(t *testing.T) {
	sqlOut := HighlightSQL("SELECT * FROM users WHERE id = 1;")
	if sqlOut == "" || !strings.Contains(sqlOut, "SELECT") {
		t.Errorf("expected highlighted SQL, got %q", sqlOut)
	}

	jsOut := HighlightJS("var data = res1;\nconsole.log(data);")
	if jsOut == "" || !strings.Contains(jsOut, "data") {
		t.Errorf("expected highlighted JS, got %q", jsOut)
	}

	emptyCode := HighlightCode("", "sql")
	if emptyCode != "" {
		t.Errorf("expected empty string for empty code, got %q", emptyCode)
	}

	fallbackCode := HighlightCode("some raw string", "unknown_lexer")
	if fallbackCode == "" {
		t.Errorf("expected fallback highlight for unknown lexer")
	}
}

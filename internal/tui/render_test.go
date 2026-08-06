package tui

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	out := RenderMarkdown("# Hello World\nThis is a **test**.", 80)
	if !strings.Contains(out, "Hello World") {
		t.Errorf("expected rendered markdown to contain 'Hello World', got %q", out)
	}

	empty := RenderMarkdown("", 80)
	if empty != "" {
		t.Errorf("expected empty string for empty input, got %q", empty)
	}

	narrow := RenderMarkdown("# Header", 5)
	if !strings.Contains(narrow, "Header") {
		t.Errorf("expected narrow width fallback, got %q", narrow)
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

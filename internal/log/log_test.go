package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)
	logger.Info("test message", "key", "value")

	out := buf.String()
	if !strings.Contains(out, "test message") {
		t.Errorf("expected 'test message' in output, got %q", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Errorf("expected 'key=value' in output, got %q", out)
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)
	logger.Warn("warning")

	out := buf.String()
	if !strings.Contains(out, "WARN") && !strings.Contains(out, "level=WARN") {
		t.Errorf("expected WARN level in output, got %q", out)
	}
}

// Package log provides logging utilities for xsql.
package log

import (
	"io"
	"log/slog"
)

// New returns a slog.Logger that writes to w (default level=INFO).
// Note: stdout is for data; logs should always be written to stderr (passed by caller).
func New(w io.Writer) *slog.Logger {
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h)
}

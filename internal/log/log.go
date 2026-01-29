package log

import (
	"io"
	"log/slog"
)

// New 返回写入到 w 的 slog.Logger（默认 level=INFO）。
// 注意：stdout=数据，日志应始终写 stderr（由调用方传入）。
func New(w io.Writer) *slog.Logger {
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h)
}

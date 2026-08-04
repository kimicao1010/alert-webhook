package api

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger 创建结构化日志 logger。
// level 支持 debug/info/warn/error，默认 info。
// format 支持 text/json，默认 text。
func NewLogger(level, format string) *slog.Logger {
	var l slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: l}
	var h slog.Handler
	if strings.ToLower(strings.TrimSpace(format)) == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

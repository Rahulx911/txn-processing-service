package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog.Logger for structured, JSON-formatted logging.
type Logger struct {
	*slog.Logger
}

// New creates a structured JSON logger at the specified level.
func New(level string) *Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: false,
	})

	return &Logger{
		Logger: slog.New(handler),
	}
}

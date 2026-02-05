package logging

import (
	"io"
	"log/slog"
	"strings"
)

// LoggerConfig holds configuration for the logger.
type LoggerConfig struct {
	Level string
}

// NewLogger creates a new slog.Logger with JSON handler and the specified output.
// The level is parsed from the config; defaults to INFO if invalid or empty.
func NewLogger(config LoggerConfig, w io.Writer) *slog.Logger {
	level := parseLevel(config.Level)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		AddSource:   false,
		Level:       level,
		ReplaceAttr: nil,
	})

	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

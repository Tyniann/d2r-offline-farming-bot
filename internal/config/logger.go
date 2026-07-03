package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewLogger builds a structured text logger for console output.
func NewLogger(level string) *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(level)})
	return slog.New(handler)
}

// NewFileLogger builds a structured text logger that writes to stdout and a log file.
func NewFileLogger(level, dir, appName string, now time.Time) (*slog.Logger, *os.File, string, error) {
	if dir == "" {
		dir = "logs"
	}
	if appName == "" {
		appName = "d2rbot"
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, "", fmt.Errorf("create log directory: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%s-%s.log", appName, now.Format("20060102-150405")))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, "", fmt.Errorf("open log file: %w", err)
	}

	handler := slog.NewTextHandler(io.MultiWriter(os.Stdout, file), &slog.HandlerOptions{Level: parseLogLevel(level)})
	return slog.New(handler), file, path, nil
}

func parseLogLevel(level string) slog.Level {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return lvl
}

package logging

import (
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger
var currentLevel slog.Level

func init() {
	// Default to INFO level
	InitLogger("info")
}

// InitLogger initializes the global logger with the specified level
func InitLogger(level string) {
	var logLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	currentLevel = logLevel

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// GetLogger returns the global logger instance
func GetLogger() *slog.Logger {
	return logger
}

// GetLevel returns the current log level
func GetLevel() slog.Level {
	return currentLevel
}

// MaskPhone masks a phone number for logging purposes, keeping the
// country code prefix and the last two digits visible
func MaskPhone(phone string) string {
	const visiblePrefix = 3
	const visibleSuffix = 2
	if len(phone) <= visiblePrefix+visibleSuffix {
		return strings.Repeat("*", len(phone))
	}
	masked := strings.Repeat("*", len(phone)-visiblePrefix-visibleSuffix)
	return phone[:visiblePrefix] + masked + phone[len(phone)-visibleSuffix:]
}

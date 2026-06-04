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
	InitLogger(slog.LevelInfo)
}

// InitLogger initializes the global logger with the specified level
func InitLogger(level slog.Level) {
	currentLevel = level

	opts := &slog.HandlerOptions{
		Level: level,
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

// MaskKey masks the phone number portion of a rate limiter key
// (e.g. "namespace:phone:+31612345678") for logging purposes.
// Keys without a phone segment are returned unchanged.
func MaskKey(key string) string {
	const marker = "phone:"
	idx := strings.LastIndex(key, marker)
	if idx == -1 {
		return key
	}
	prefix := key[:idx+len(marker)]
	return prefix + MaskPhone(key[idx+len(marker):])
}

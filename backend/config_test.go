package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestReadConfigFileLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectedLevel slog.Level
	}{
		{"debug", `{"log_level": "debug"}`, slog.LevelDebug},
		{"info", `{"log_level": "info"}`, slog.LevelInfo},
		{"warn", `{"log_level": "warn"}`, slog.LevelWarn},
		{"error", `{"log_level": "error"}`, slog.LevelError},
		{"uppercase", `{"log_level": "DEBUG"}`, slog.LevelDebug},
		{"mixed case", `{"log_level": "InFo"}`, slog.LevelInfo},
		{"missing defaults to info", `{}`, slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := readConfigFile(writeConfig(t, tt.config))
			require.NoError(t, err)
			require.Equal(t, tt.expectedLevel, config.LogLevel)
		})
	}
}

func TestReadConfigFileRejectsUnknownLogLevel(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{"typo", `{"log_level": "infor"}`},
		{"unsupported level", `{"log_level": "trace"}`},
		{"empty string", `{"log_level": ""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readConfigFile(writeConfig(t, tt.config))
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown name")
		})
	}
}

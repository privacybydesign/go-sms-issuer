package logging

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultLoggerInitialized(t *testing.T) {
	logger := GetLogger()
	require.NotNil(t, logger, "Logger should be initialized")
}

func TestInitLoggerWithDifferentLevels(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedLevel slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"warning level", "warning", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"default for unknown", "invalid", slog.LevelInfo},
		{"empty string defaults to info", "", slog.LevelInfo},
		{"uppercase", "DEBUG", slog.LevelDebug},
		{"mixed case", "InFo", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitLogger(tt.level)
			logger := GetLogger()
			require.NotNil(t, logger)

			// Verify the level is set correctly
			actualLevel := GetLevel()
			require.Equal(t, tt.expectedLevel, actualLevel,
				"Expected level %v but got %v for input %q",
				tt.expectedLevel, actualLevel, tt.level)
		})
	}
}

func TestGetLogger(t *testing.T) {
	InitLogger("info")
	logger1 := GetLogger()
	logger2 := GetLogger()

	require.NotNil(t, logger1)
	require.NotNil(t, logger2)
	require.Equal(t, logger1, logger2, "GetLogger should return the same instance")
}

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		phone    string
		expected string
	}{
		{"+31612345678", "+31*******78"},
		{"0031612345678", "003********78"},
		{"12345", "*****"},
		{"", ""},
	}

	for _, tt := range tests {
		require.Equal(t, tt.expected, MaskPhone(tt.phone))
	}
}

func TestInitLoggerWarnsOnUnknownLevel(t *testing.T) {
	tests := []struct {
		name       string
		level      string
		expectWarn bool
	}{
		{"typo warns", "infor", true},
		{"unsupported level warns", "trace", true},
		{"valid level is silent", "debug", false},
		{"empty string is silent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStderr(t, func() { InitLogger(tt.level) })
			if tt.expectWarn {
				require.Contains(t, output, "unknown log level, defaulting to info")
				require.Contains(t, output, tt.level)
			} else {
				require.NotContains(t, output, "unknown log level")
			}
		})
	}
}

// captureStderr runs fn while redirecting os.Stderr and returns what was written
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"sms:phone:+31612345678", "sms:phone:+31*******78"},
		{"phone:+31612345678", "phone:+31*******78"},
		{"sms:ip:1.2.3.4", "sms:ip:1.2.3.4"},
		{"ip:1.2.3.4", "ip:1.2.3.4"},
		{"", ""},
	}

	for _, tt := range tests {
		require.Equal(t, tt.expected, MaskKey(tt.key))
	}
}

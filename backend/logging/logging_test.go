package logging

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultLoggerInitialized(t *testing.T) {
	logger := GetLogger()
	require.NotNil(t, logger, "Logger should be initialized")
}

func TestInitLoggerWithDifferentLevels(t *testing.T) {
	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			InitLogger(level)
			logger := GetLogger()
			require.NotNil(t, logger)
			require.Equal(t, level, GetLevel())
		})
	}
}

func TestGetLogger(t *testing.T) {
	InitLogger(slog.LevelInfo)
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

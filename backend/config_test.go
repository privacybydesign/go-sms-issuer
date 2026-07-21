package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"go-sms-issuer/altcha"

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

func TestCreateAltchaVerifierDerivedFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		wantState altcha.EnforcementState
		wantErr   bool
	}{
		{"no altcha_backend disables", `{"storage_type": "memory"}`, altcha.Disabled, false},
		{"explicit disabled", `{"storage_type": "memory", "altcha_backend": "disabled"}`, altcha.Disabled, false},
		{"monitor needs secret", `{"storage_type": "memory", "altcha_backend": "monitor"}`, altcha.Disabled, true},
		{"enforced needs secret", `{"storage_type": "memory", "altcha_backend": "enforced"}`, altcha.Disabled, true},
		{"monitor with secret", `{"storage_type": "memory", "altcha_backend": "monitor", "altcha_config": {"secret": "long-random-secret"}}`, altcha.Monitor, false},
		{"enforced with secret", `{"storage_type": "memory", "altcha_backend": "enforced", "altcha_config": {"secret": "long-random-secret"}}`, altcha.Enforced, false},
		{"invalid backend", `{"storage_type": "memory", "altcha_backend": "bogus"}`, altcha.Disabled, true},
		{"wrong algorithm rejected", `{"storage_type": "memory", "altcha_backend": "enforced", "altcha_config": {"secret": "s", "algorithm": "PBKDF2/SHA-512"}}`, altcha.Disabled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := readConfigFile(writeConfig(t, tt.config))
			require.NoError(t, err)

			verifier, err := createAltchaVerifier(&config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantState, verifier.State())
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

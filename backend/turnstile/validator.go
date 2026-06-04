package turnstile

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

type TurnStileVerifier interface {
	Verify(token string, ip string) bool
}

// Configuration struct
type TurnStileConfiguration struct {
	SecretKey string `json:"secret_key"`
	SiteKey   string `json:"site_key"`
	ApiUrl    string `json:"api_url"`
}

// Concrete production implementation
type TurnStileValidator struct {
	SecretKey string
	ApiUrl    string
}

// Constructor for production use
func NewTurnStileValidator(cfg TurnStileConfiguration) *TurnStileValidator {
	return &TurnStileValidator{
		SecretKey: cfg.SecretKey,
		ApiUrl:    cfg.ApiUrl,
	}
}

// Actual verification logic for Turnstile
func (v *TurnStileValidator) Verify(token string, ip string) bool {
	values := url.Values{}
	values.Set("secret", v.SecretKey)
	values.Set("response", token)
	if ip != "" {
		values.Set("remoteip", ip)
	}

	resp, err := http.PostForm(v.ApiUrl, values)
	if err != nil {
		slog.Error("turnstile request failed", "error", err)
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed reading turnstile response", "error", err)
		return false
	}

	var data struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		slog.Error("failed parsing turnstile response", "error", err)
		return false
	}

	return data.Success
}

// Mock validator for testing
type MockTurnStileValidator struct {
	Success bool
}

// Mock behavior
func (m *MockTurnStileValidator) Verify(token string, ip string) bool {
	slog.Info("MockTurnStileValidator called", "token", token, "ip", ip)
	return m.Success
}

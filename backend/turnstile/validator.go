package turnstile

import (
	"encoding/json"
	log "go-sms-issuer/logging"
	"io"
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
		log.Error.Printf("turnstile request failed: %v", err)
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Error.Printf("failed to close request body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error.Printf("failed reading turnstile response: %v", err)
		return false
	}

	var data struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Error.Printf("failed parsing turnstile response: %v", err)
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
	log.Info.Printf("MockTurnStileValidator called with token: %s, ip: %s", token, ip)
	return m.Success
}

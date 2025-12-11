package altcha

import (
	"encoding/base64"
	"encoding/json"
	altcha "github.com/altcha-org/altcha-lib-go"
	log "go-sms-issuer/logging"
)

type ChallengeVerifier interface {
	CreateChallenge() (Challenge, error)
	Verify(payload string) bool
}

// Challenge represents an ALTCHA challenge
type Challenge struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	MaxNumber int    `json:"maxnumber"`
	Salt      string `json:"salt"`
	Signature string `json:"signature"`
}

// Configuration struct
type Configuration struct {
	HMACKey string `json:"hmac_key"`
}

// Concrete production implementation
type Validator struct {
	HMACKey string
}

// Constructor for production use
func NewValidator(cfg Configuration) *Validator {
	return &Validator{
		HMACKey: cfg.HMACKey,
	}
}

// CreateChallenge generates a new ALTCHA challenge
func (v *Validator) CreateChallenge() (Challenge, error) {
	challenge, err := altcha.CreateChallenge(altcha.ChallengeOptions{
		HMACKey:   v.HMACKey,
		MaxNumber: 50000,
	})

	if err != nil {
		log.Error.Printf("failed to create altcha challenge: %v", err)
		return Challenge{}, err
	}

	return Challenge{
		Algorithm: challenge.Algorithm,
		Challenge: challenge.Challenge,
		MaxNumber: int(challenge.MaxNumber),
		Salt:      challenge.Salt,
		Signature: challenge.Signature,
	}, nil
}

// Verify validates an ALTCHA solution
func (v *Validator) Verify(payload string) bool {
	// Decode the base64-encoded payload
	decodedPayload, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		log.Error.Printf("failed to decode altcha payload: %v", err)
		return false
	}

	// Parse the JSON payload
	var data map[string]interface{}
	if err := json.Unmarshal(decodedPayload, &data); err != nil {
		log.Error.Printf("failed to parse altcha payload: %v", err)
		return false
	}

	// Verify the solution
	verified, err := altcha.VerifySolution(data, v.HMACKey, true)
	if err != nil {
		log.Error.Printf("altcha verification failed: %v", err)
		return false
	}

	return verified
}

// Mock validator for testing
type MockValidator struct {
	Success bool
}

// CreateChallenge mock implementation
func (m *MockValidator) CreateChallenge() (Challenge, error) {
	log.Info.Printf("MockValidator.CreateChallenge called")
	return Challenge{
		Algorithm: "SHA-256",
		Challenge: "mock-challenge",
		MaxNumber: 50000,
		Salt:      "mock-salt",
		Signature: "mock-signature",
	}, nil
}

// Mock behavior
func (m *MockValidator) Verify(payload string) bool {
	log.Info.Printf("MockValidator called with payload: %s", payload)
	return m.Success
}

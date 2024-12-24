package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

type mockSmsSender struct {
	sent map[string]string
}

func newMockSmsSender() *mockSmsSender {
	return &mockSmsSender{
		sent: make(map[string]string),
	}
}

func (m *mockSmsSender) SendSms(phone, message string) error {
	return nil
}

type mockJwtCreator struct{}

func (m *mockJwtCreator) CreateJwt(phone string) (string, error) {
	return "JWT", nil
}

func createAndStartTestServer(t *testing.T) *Server {
	smsSender := newMockSmsSender()

	state := ServerState{
		tokenRepo:      NewInMemoryTokenRepo(),
		smsSender:      smsSender,
		jwtCreator:     &mockJwtCreator{},
		tokenGenerator: &DefaultTokenGenerator{},
		smsTemplates: map[string]string{
			"en": "your token: %v",
		},
	}

	config := ServerConfig{
		Host:   "127.0.0.1",
		Port:   8081,
		UseTls: false,
	}

	server, err := NewServer(state, config)

	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			ErrorLogger.Printf("server crashed: %v", err)
		}
	}()
	return server
}

func makeSendSmsRequest(phone, language string) (*http.Response, error) {
	payload := SendSmsPayload{
		PhoneNumber: phone,
		Language:    language,
	}
	json, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return http.Post("http://127.0.0.1:8081/send", "application/json", bytes.NewBuffer(json))
}

func TestSendingSendRequest(t *testing.T) {
	server := createAndStartTestServer(t)
	defer server.Stop()

	resp, err := makeSendSmsRequest("+31612345678", "en")

	if err != nil {
		t.Fatalf("making a response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code not ok: %v", resp.StatusCode)
	} else {
		t.Log("Test completed successfully")
	}
}

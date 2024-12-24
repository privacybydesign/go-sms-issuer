package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestWrongPhoneNumberFails(t *testing.T) {
	server := createAndStartTestServer(t)
	defer server.Stop()

    phone := "+31612345678"

    resp, err := makeVerifyRequest(phone, "123456")
    if err != nil {
        t.Fatalf("failed to send verify request")
    }

    if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf(
			"status code was expected to be %v (BadRequest), but was %v",
			http.StatusBadRequest,
			resp.StatusCode,
		)
    }
}

func TestWrongTokenFails(t *testing.T) {
	server := createAndStartTestServer(t)
	defer server.Stop()

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en")

	if err != nil {
		t.Fatalf("send sms request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send-sms response status code not ok: %v", resp.StatusCode)
	}

	resp, err = makeVerifyRequest(phone, "111111")

	if err != nil {
		t.Fatalf("verify request failed: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf(
			"status code was expected to be %v (unauthorized), but was %v",
			http.StatusUnauthorized,
			resp.StatusCode,
		)
	}
}

func TestSendingSendRequest(t *testing.T) {
	server := createAndStartTestServer(t)
	defer server.Stop()

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en")

	if err != nil {
		t.Fatalf("send sms request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send-sms response status code not ok: %v", resp.StatusCode)
	}

	resp, err = makeVerifyRequest(phone, "123456")

	if err != nil {
		t.Fatalf("verify request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify response status code not OK: %v", resp.StatusCode)
	}

	body, err := readCompleteBodyToString(resp)
	if body != "JWT" {
		t.Fatalf("body not JWT: %v, %v", err, body)
	}
}

// ------------------------------------------------------------------------

func readCompleteBodyToString(r *http.Response) (string, error) {
	defer r.Body.Close()
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

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
		// if server didn't close succesfully
		if err != http.ErrServerClosed {
			ErrorLogger.Fatalf("server crashed: %v", err)
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

func makeVerifyRequest(phone, token string) (*http.Response, error) {
	payload := VerifyPayload{
		PhoneNumber: phone,
		Token:       token,
	}
	json, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return http.Post("http://127.0.0.1:8081/verify", "application/json", bytes.NewBuffer(json))
}

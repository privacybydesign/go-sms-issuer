package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
)

// for testing purposes it's useful to have a static token
const testToken = "123456"

func TestRateLimitingSingleClient(t *testing.T) {
	server := createAndStartTestServer(t, nil)
	defer server.Stop()

	phone := "+31612345678"

	for i := 1; i <= 3; i++ {
		// first request should be fine
		resp, err := makeSendSmsRequest(phone, "en")
		if err != nil {
			t.Fatalf("failed to send sms request: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %v failed, where it should succeed: %v", i, resp.StatusCode)
		}
	}

	// third request should be getting rate limited
	resp, err := makeSendSmsRequest(phone, "en")
	if err != nil {
		t.Fatalf("failed to send sms request: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("fourth request was expected to be rate limited: %v", resp.StatusCode)
	}
}

func TestUnsupportedLanguageFails(t *testing.T) {
	server := createAndStartTestServer(t, nil)
	defer server.Stop()

	phone := "+31687654321"

	resp, err := makeSendSmsRequest(phone, "fr")
	if err != nil {
		t.Fatalf("failed to send sms request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf(
			"status code was expected to be %v (BadRequest), but was %v",
			http.StatusBadRequest,
			resp.StatusCode,
		)
	}
}

func TestSmsIsBeingSent(t *testing.T) {
	// channel must be buffered
	smsReceiver := make(chan smsMessage, 1)
	server := createAndStartTestServer(t, &smsReceiver)
	defer server.Stop()

	phone := "+31687654321"

	resp, err := makeSendSmsRequest(phone, "en")
	if err != nil {
		t.Fatalf("failed to send sms request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("response not ok: %v", resp.StatusCode)
	}

	sms := <-smsReceiver
	if sms.phone != phone {
		t.Fatalf("not sending sms to correct phone number: %v instead of %v", sms.phone, phone)
	}
	if !strings.Contains(sms.message, testToken) {
		t.Fatalf("sms message doesn't contain token: %v", sms.message)
	}
}

func TestVerifyWrongPhoneNumberFails(t *testing.T) {
	server := createAndStartTestServer(t, nil)
	defer server.Stop()

	phone := "+31612345678"

	resp, err := makeVerifyRequest(phone, testToken)
	if err != nil {
		t.Fatalf("failed to send verify request: %v", err)
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
	server := createAndStartTestServer(t, nil)
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
			"status code was expected to be %v (Unauthorized), but was %v",
			http.StatusUnauthorized,
			resp.StatusCode,
		)
	}
}

func TestSendingSendRequest(t *testing.T) {
	server := createAndStartTestServer(t, nil)
	defer server.Stop()

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en")

	if err != nil {
		t.Fatalf("send sms request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send-sms response status code not ok: %v", resp.StatusCode)
	}

	resp, err = makeVerifyRequest(phone, testToken)

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

// message sent by the sms sender
type smsMessage struct {
	phone, message string
}

type mockSmsSender struct {
	sendTo *chan smsMessage
}

func newMockSmsSender(ch *chan smsMessage) *mockSmsSender {
	return &mockSmsSender{
		sendTo: ch,
	}
}

func (m *mockSmsSender) SendSms(phone, mess string) error {
	if m.sendTo == nil {
		return nil
	}
	msg := smsMessage{
		phone: phone, message: mess,
	}
	(*m.sendTo) <- msg
	return nil
}

type mockJwtCreator struct{}

func (m *mockJwtCreator) CreateJwt(phone string) (string, error) {
	return "JWT", nil
}

func createAndStartTestServer(t *testing.T, smsChan *chan smsMessage) *Server {
	smsSender := newMockSmsSender(smsChan)

	state := ServerState{
		tokenStorage:   NewInMemoryTokenStorage(),
		smsSender:      smsSender,
		jwtCreator:     &mockJwtCreator{},
		tokenGenerator: &StaticTokenGenerator{token: testToken},
		smsTemplates: map[string]string{
			"en": "your token: %v",
		},
		rateLimiter: rate.NewTotalRateLimiter(
			rate.NewRateLimiter(rate.NewInMemoryRateLimiterStorage(), rate.NewSystemClock(), rate.DefaultTimeoutPolicy),
			rate.NewRateLimiter(rate.NewInMemoryRateLimiterStorage(), rate.NewSystemClock(), rate.DefaultTimeoutPolicy),
		),
	}

	config := ServerConfig{
		Host:   "0.0.0.0",
		Port:   8081,
		UseTls: false,
	}

	server, err := NewServer(&state, config)

	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	go func() {
		err := server.ListenAndServe()
		// if server didn't close succesfully
		if err != http.ErrServerClosed {
			log.Error.Fatalf("server crashed: %v", err)
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

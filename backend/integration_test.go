package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	turnstile "go-sms-issuer/turnstile"
)

// for testing purposes it's useful to have a static token
const testToken = "123456"
const testCaptcha = "test-captcha"

func TestEmptyCaptcha(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	// first request should be fine
	resp, err := makeSendSmsRequest(phone, "en", "")
	if err != nil {
		t.Fatalf("failed to send sms request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected BadRequest for missing captcha, got: %v", resp.StatusCode)
	}
}

func TestWithCaptchaButFailed(t *testing.T) {
	server := createAndStartTestServer(t, nil, false)
	defer stopServer(server)

	phone := "+31612345678"

	// first request should be fine
	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
	if err != nil {
		t.Fatalf("failed to send sms request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected BadRequest for missing captcha, got: %v", resp.StatusCode)
	}
}

func TestRateLimitingSingleClient(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	for i := 1; i <= 5; i++ {
		// first request should be fine
		resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
		if err != nil {
			t.Fatalf("failed to send sms request: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %v failed, where it should succeed: %v", i, resp.StatusCode)
		}
	}

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
	if err != nil {
		t.Fatalf("failed to send sms request: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("6th request was expected to be rate limited: %v", resp.StatusCode)
	}
}

func TestUnsupportedLanguageFails(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31687654321"

	resp, err := makeSendSmsRequest(phone, "fr", testCaptcha)
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
	server := createAndStartTestServer(t, &smsReceiver, true)
	defer stopServer(server)

	phone := "+31687654321"

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
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
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

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
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)

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
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)

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
	// check if body is json with JWT in it
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	// Deserialize json
	var data map[string]string
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	// Check if JWT is present
	jwt, ok := data["jwt"]
	if !ok {
		t.Fatalf("response body does not contain JWT: %v", body)
	}
	if jwt != "JWT" {
		t.Fatalf("unexpected JWT in response body: %v", jwt)
	}
	// Check if irma_server_url is present
	irmaServerURL, ok := data["irma_server_url"]
	if !ok {
		t.Fatalf("response body does not contain irma_server_url: %v", body)
	}
	if irmaServerURL != "http://localhost:8080" {
		t.Fatalf("unexpected irma_server_url in response body: %v", irmaServerURL)
	}
}

// ------------------------------------------------------------------------

func readCompleteBodyToString(r *http.Response) (string, error) {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	err = r.Body.Close()
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

func NewMockTurnStileVerifier(turnstileSuccess bool) turnstile.TurnStileVerifier {
	return &turnstile.MockTurnStileValidator{Success: turnstileSuccess}
}

type mockJwtCreator struct{}

func (m *mockJwtCreator) CreateJwt(phone string) (string, error) {
	return "JWT", nil
}

func createAndStartTestServer(t *testing.T, smsChan *chan smsMessage, turnstileSuccess bool) *Server {
	smsSender := newMockSmsSender(smsChan)

	turnstileVerifier := NewMockTurnStileVerifier(turnstileSuccess)

	ipRateLimitingPolicy := rate.RateLimitingPolicy{
		Window: time.Minute * 30,
		Limit:  10,
	}

	phoneLimitPolicy := rate.RateLimitingPolicy{
		Window: time.Minute * 30,
		Limit:  5,
	}

	state := ServerState{
		irmaServerURL:  "http://localhost:8080",
		tokenStorage:   NewInMemoryTokenStorage(),
		smsSender:      smsSender,
		jwtCreator:     &mockJwtCreator{},
		tokenGenerator: &StaticTokenGenerator{token: testToken},
		smsTemplates: map[string]string{
			"en": "your token: %v",
		},
		rateLimiter: rate.NewTotalRateLimiter(
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), ipRateLimitingPolicy),
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), phoneLimitPolicy),
		),
		turnstileVerifier: turnstileVerifier,
	}

	config := ServerConfig{
		Host:   "127.0.0.1",
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

	// Wait for server to be ready
	const maxAttempts = 50
	for i := 0; i < maxAttempts; i++ {
		resp, err := http.Get("http://localhost:8081/")
		if err == nil {
			err = resp.Body.Close()
			if err != nil {
				t.Fatalf("error closing response body: %v", err)
			}
			break
		}
		// Wait 50ms before retrying
		time.Sleep(50 * time.Millisecond)
		if i == maxAttempts-1 {
			t.Fatalf("server did not start in time: %v", err)
		}
	}

	return server
}

func makeSendSmsRequest(phone, language string, captcha string) (*http.Response, error) {
	payload := SendSmsPayload{
		PhoneNumber: phone,
		Language:    language,
		Captcha:     captcha,
	}
	json, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return http.Post("http://localhost:8081/send", "application/json", bytes.NewBuffer(json))
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
	return http.Post("http://localhost:8081/verify", "application/json", bytes.NewBuffer(json))
}

func stopServer(server *Server) {
	err := server.Stop()
	if err != nil {
		log.Error.Printf("error shutting down server: %v", err)
	}
}

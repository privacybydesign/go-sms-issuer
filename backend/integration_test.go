package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	turnstile "go-sms-issuer/turnstile"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestWithCaptchaButFailed(t *testing.T) {
	server := createAndStartTestServer(t, nil, false)
	defer stopServer(server)

	phone := "+31612345678"

	// first request should be fine
	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestRateLimitingVerifyCode(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	_, err := makeSendSmsRequest(phone, "en", testCaptcha)
	require.NoError(t, err)

	// first 25 should fail but should not be limited
	for i := 1; i <= 25; i++ {
		resp, err := makeVerifyRequest(phone, "en")
		require.NoError(t, err)
		require.Equal(t, resp.StatusCode, http.StatusOK, "failed at request %v", i)
	}

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusTooManyRequests)
}

func TestRateLimitingSingleClient(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	for i := 1; i <= 5; i++ {
		resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
		require.NoError(t, err)
		require.Equal(t, resp.StatusCode, http.StatusOK, "failed at request %v", i)
	}

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusTooManyRequests)
}

func TestUnsupportedLanguageFails(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31687654321"

	resp, err := makeSendSmsRequest(phone, "fr", testCaptcha)
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusBadRequest)
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
	require.Equal(t, sms.phone, phone)
	require.Contains(t, sms.message, testToken)
}

func TestVerifyWrongPhoneNumberFails(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	resp, err := makeVerifyRequest(phone, testToken)
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestWrongTokenFails(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)

	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusOK)

	resp, err = makeVerifyRequest(phone, "111111")
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusUnauthorized)
}

func TestSendingSendRequest(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusOK)

	resp, err = makeVerifyRequest(phone, testToken)

	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusOK)

	body, err := readCompleteBodyToString(resp)
	require.NoError(t, err)

	// check if body is json with JWT in it
	// Deserialize json
	var data map[string]string
	err = json.Unmarshal([]byte(body), &data)
	require.NoError(t, err)
	// Check if JWT is present
	jwt, ok := data["jwt"]
	require.True(t, ok, "response body does not contain JWT: %v")
	require.Equal(t, jwt, "JWT")

	// Check if irma_server_url is present
	irmaServerURL, ok := data["irma_server_url"]
	require.True(t, ok, "response body does not contain irma_server_url: %v", body)
	require.Equal(t, irmaServerURL, "http://localhost:8080")
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
		sendSmsRateLimiter: rate.NewTotalRateLimiter(
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
	require.NoError(t, err)

	go func() {
		err := server.ListenAndServe()
		// if server didn't close succesfully -> fail
		require.Equal(t, err, http.ErrServerClosed)
	}()

	// Wait for server to be ready
	const maxAttempts = 50
	for i := 0; i < maxAttempts; i++ {
		resp, err := http.Get("http://localhost:8081/")
		if err == nil {
			err = resp.Body.Close()
			require.NoError(t, err)
			break
		}
		// Wait 50ms before retrying
		time.Sleep(50 * time.Millisecond)
		require.NotEqual(t, i, maxAttempts-1, "server did not start in time: %v", err)
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

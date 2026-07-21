package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"go-sms-issuer/altcha"
	rate "go-sms-issuer/rate_limiter"
	turnstile "go-sms-issuer/turnstile"

	altchalib "github.com/altcha-org/altcha-lib-go/v2"
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

	// The rate limiter check runs before token lookup, so all 25 attempts
	// tick the counter regardless of whether the token is still around.
	// After MaxFailedAttempts wrong submissions the token is invalidated,
	// so the response transitions from 401 ("token incorrect") to 400
	// ("no active token request") — but the rate limit cap is unchanged.
	for i := 1; i <= 25; i++ {
		resp, err := makeVerifyRequest(phone, "000000")
		require.NoError(t, err)
		if i <= MaxFailedAttempts {
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "failed at request %v", i)
		} else {
			require.Equal(t, http.StatusBadRequest, resp.StatusCode, "failed at request %v", i)
		}
	}

	// 26th should be limited
	resp, err := makeVerifyRequest(phone, "000000")
	require.NoError(t, err)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestTokenInvalidatedAfterMaxFailedAttempts(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	phone := "+31612345678"

	resp, err := makeSendSmsRequest(phone, "en", testCaptcha)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The first MaxFailedAttempts wrong submissions return 401.
	for i := 1; i <= MaxFailedAttempts; i++ {
		resp, err := makeVerifyRequest(phone, "000000")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "failed at request %v", i)
	}

	// After that the token is removed, so even the correct code can no
	// longer verify against this /send and we get the "no active token"
	// branch instead.
	resp, err = makeVerifyRequest(phone, testToken)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
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
// Embedded issuance ALTCHA proof of work

func newTestAltchaVerifier(t *testing.T, state altcha.EnforcementState) *altcha.HmacVerifier {
	t.Helper()
	// A tiny cost and single-byte prefix keep the round-trip fast in tests
	// while still exercising the full create/solve/verify path.
	v, err := altcha.NewHmacVerifier(state, "test-secret", 50, 1, time.Minute, altcha.NewInMemorySeenStore())
	require.NoError(t, err)
	return v
}

func TestEmbeddedSendWorksWithoutAltchaWhenDisabled(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	resp, err := makeEmbeddedSendRequest("+31612345678", "en")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEmbeddedAltchaChallengeReturns404WhenDisabled(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	resp, err := http.Get("http://localhost:8081/api/embedded/altcha-challenge")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestEmbeddedSendRejectsMissingAltchaWhenEnforced(t *testing.T) {
	server := createAndStartTestServer(t, nil, true, newTestAltchaVerifier(t, altcha.Enforced))
	defer stopServer(server)

	resp, err := makeEmbeddedSendRequest("+31612345678", "en")
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := readCompleteBodyToString(resp)
	require.NoError(t, err)
	require.Equal(t, ErrorInvalidCaptcha, body)
}

func TestEmbeddedAltchaChallengeAndSendHappyPath(t *testing.T) {
	server := createAndStartTestServer(t, nil, true, newTestAltchaVerifier(t, altcha.Enforced))
	defer stopServer(server)

	payload := solveAltchaChallenge(t)

	resp, err := makeEmbeddedSendRequest("+31612345678", "en", payload)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEmbeddedAltchaSolutionCannotBeReplayed(t *testing.T) {
	server := createAndStartTestServer(t, nil, true, newTestAltchaVerifier(t, altcha.Enforced))
	defer stopServer(server)

	payload := solveAltchaChallenge(t)

	resp, err := makeEmbeddedSendRequest("+31612345678", "en", payload)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Reusing the same solved challenge must be rejected.
	resp, err = makeEmbeddedSendRequest("+31612345678", "en", payload)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestEmbeddedSendAllowsMissingAltchaInMonitorMode(t *testing.T) {
	// Monitor mode hands out challenges but still accepts a send that lacks a
	// solution, so old apps keep working during the measured grace window.
	server := createAndStartTestServer(t, nil, true, newTestAltchaVerifier(t, altcha.Monitor))
	defer stopServer(server)

	// The challenge endpoint is live in monitor mode.
	resp, err := http.Get("http://localhost:8081/api/embedded/altcha-challenge")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// A send without a solution is still accepted.
	resp, err = makeEmbeddedSendRequest("+31612345678", "en")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// solveAltchaChallenge fetches a challenge from the running server and returns a
// base64-encoded solved ALTCHA payload, mirroring what the app client submits.
func solveAltchaChallenge(t *testing.T) string {
	t.Helper()
	resp, err := http.Get("http://localhost:8081/api/embedded/altcha-challenge")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var challenge altchalib.Challenge
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&challenge))

	solution, err := altchalib.SolveChallenge(altchalib.SolveChallengeOptions{
		Challenge: challenge,
		DeriveKey: altchalib.DeriveKeyPBKDF2(),
	})
	require.NoError(t, err)
	require.NotNil(t, solution)

	raw, err := json.Marshal(altchalib.Payload{Challenge: challenge, Solution: *solution})
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
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

func createAndStartTestServer(t *testing.T, smsChan *chan smsMessage, turnstileSuccess bool, altchaVerifiers ...altcha.Verifier) *Server {
	smsSender := newMockSmsSender(smsChan)

	turnstileVerifier := NewMockTurnStileVerifier(turnstileSuccess)

	// The embedded proof of work is disabled unless a test opts in by passing a
	// verifier, so existing tests keep exercising the captcha-free path.
	var altchaVerifier altcha.Verifier = altcha.DisabledVerifier{}
	if len(altchaVerifiers) > 0 {
		altchaVerifier = altchaVerifiers[0]
	}

	sendSmsIpRateLimitingPolicy := rate.RateLimitingPolicy{
		Window: time.Minute * 30,
		Limit:  10,
	}

	sendSmsPhoneRateLimitPolicy := rate.RateLimitingPolicy{
		Window: time.Minute * 30,
		Limit:  5,
	}

	verifyCodeIpRateLimitingPolicy := rate.RateLimitingPolicy{
		Window: time.Minute * 30,
		Limit:  25,
	}

	verifyCodePhoneRateLimitPolicy := rate.RateLimitingPolicy{
		Window: time.Minute * 30,
		Limit:  25,
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
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), sendSmsIpRateLimitingPolicy),
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), sendSmsPhoneRateLimitPolicy),
		),
		verifyCodeRateLimiter: rate.NewTotalRateLimiter(
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), verifyCodeIpRateLimitingPolicy),
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), verifyCodePhoneRateLimitPolicy),
		),
		turnstileVerifier: turnstileVerifier,
		altchaVerifier:    altchaVerifier,
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
	for i := range maxAttempts {
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
		slog.Error("error shutting down server", "error", err)
	}
}

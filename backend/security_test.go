package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// makeEmbeddedSendRequest posts to the captcha-free embedded endpoint with the
// given bearer token in the Authorization header (omitted when empty).
func makeEmbeddedSendRequest(phone, language, authToken string) (*http.Response, error) {
	payload := EmbeddedIssuance_SendSmsPayload{
		PhoneNumber: phone,
		Language:    language,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8081/api/embedded/send", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	return http.DefaultClient.Do(req)
}

func TestEmbeddedSendRejectsMissingAuth(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	resp, err := makeEmbeddedSendRequest("+31612345678", "en", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestEmbeddedSendRejectsWrongAuth(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	resp, err := makeEmbeddedSendRequest("+31612345678", "en", "not-the-secret")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestEmbeddedSendAcceptsValidAuth(t *testing.T) {
	smsReceiver := make(chan smsMessage, 1)
	server := createAndStartTestServer(t, &smsReceiver, true)
	defer stopServer(server)

	resp, err := makeEmbeddedSendRequest("+31612345678", "en", testEmbeddedAuthToken)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	sms := <-smsReceiver
	require.Contains(t, sms.message, testToken)
}

func TestEmbeddedSendRejectsInvalidPhoneWithValidAuth(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	resp, err := makeEmbeddedSendRequest("0612345678", "en", testEmbeddedAuthToken)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body, err := readCompleteBodyToString(resp)
	require.NoError(t, err)
	require.Equal(t, ErrorPhoneNumberFormat, body)
}

func TestSendRejectsInvalidPhoneNumber(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	// not E.164: no leading '+', contains letters, or empty
	for _, bad := range []string{"0612345678", "+0612345678", "abc", "", "+"} {
		resp, err := makeSendSmsRequest(bad, "en", testCaptcha)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "phone %q should be rejected", bad)
		body, err := readCompleteBodyToString(resp)
		require.NoError(t, err)
		require.Equal(t, ErrorPhoneNumberFormat, body, "phone %q", bad)
	}
}

func TestSendBodyTooLargeIsRejected(t *testing.T) {
	server := createAndStartTestServer(t, nil, true)
	defer stopServer(server)

	// a body well above the 4 KiB cap should be rejected before parsing
	oversized := strings.Repeat("a", maxRequestBodyBytes+1)
	payload := map[string]string{"phone": "+31612345678", "language": "en", "captcha": testCaptcha, "padding": oversized}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post("http://localhost:8081/send", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---- unit tests for the IP resolution and E.164 helpers ----

func mustCIDRs(t *testing.T, cidrs ...string) []*net.IPNet {
	t.Helper()
	nets, err := parseTrustedProxies(cidrs)
	require.NoError(t, err)
	return nets
}

func TestGetIpUntrustedPeerIgnoresXRealIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/send", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Real-IP", "1.2.3.4")

	// no trusted proxies configured -> X-Real-IP is never honoured
	require.Equal(t, "203.0.113.9", getIpAddressForRequest(r, nil))
}

func TestGetIpTrustedProxyHonoursXRealIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/send", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Real-IP", "1.2.3.4")

	trusted := mustCIDRs(t, "10.0.0.0/8")
	require.Equal(t, "1.2.3.4", getIpAddressForRequest(r, trusted))
}

func TestGetIpTrustedProxyWithoutHeaderUsesPeer(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/send", nil)
	r.RemoteAddr = "10.0.0.5:5555"

	trusted := mustCIDRs(t, "10.0.0.0/8")
	require.Equal(t, "10.0.0.5", getIpAddressForRequest(r, trusted))
}

func TestGetIpUntrustedPeerWithProxyConfiguredIgnoresHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/send", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Real-IP", "1.2.3.4")

	// peer is not in the trusted range -> spoofed header is ignored
	trusted := mustCIDRs(t, "10.0.0.0/8")
	require.Equal(t, "203.0.113.9", getIpAddressForRequest(r, trusted))
}

func TestHasValidEmbeddedAuth(t *testing.T) {
	newReq := func(header string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/embedded/send", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		return r
	}

	require.True(t, hasValidEmbeddedAuth(newReq("Bearer secret"), "secret"))
	require.False(t, hasValidEmbeddedAuth(newReq("Bearer wrong"), "secret"))
	require.False(t, hasValidEmbeddedAuth(newReq("secret"), "secret"), "missing Bearer prefix")
	require.False(t, hasValidEmbeddedAuth(newReq(""), "secret"), "missing header")
	// fail closed: an unconfigured token rejects everything
	require.False(t, hasValidEmbeddedAuth(newReq("Bearer secret"), ""))
}

func TestIsValidE164(t *testing.T) {
	valid := []string{"+31612345678", "+1650253000", "+12"}
	for _, v := range valid {
		require.True(t, isValidE164(v), "%q should be valid", v)
	}
	invalid := []string{"", "+", "0612345678", "+0612345678", "31612345678", "+31 612345678", "+abc", "+123456789012345678"}
	for _, v := range invalid {
		require.False(t, isValidE164(v), "%q should be invalid", v)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	nets, err := parseTrustedProxies([]string{"10.0.0.0/8", "192.168.1.1", " ", "::1/128"})
	require.NoError(t, err)
	require.Len(t, nets, 3)

	_, err = parseTrustedProxies([]string{"not-a-cidr"})
	require.Error(t, err)
}

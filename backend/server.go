package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	turnstile "go-sms-issuer/turnstile"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
)

// same error message bodies as the old Java code
const ErrorPhoneNumberFormat = "error:phone-number-format"
const ErrorRateLimit = "error:ratelimit"
const ErrorCannotValidateToken = "error:cannot-validate-token"
const ErrorAddressMalformed = "error:address-malformed"
const ErrorInternal = "error:internal"
const ErrorSendingSms = "error:sending-sms"
const ErrorInvalidCaptcha = "error:invalid-captcha"

type ServerConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	UseTls         bool   `json:"use_tls,omitempty"`
	TlsPrivKeyPath string `json:"tls_priv_key_path,omitempty"`
	TlsCertPath    string `json:"tls_cert_path,omitempty"`
}

type ServerState struct {
	irmaServerURL         string
	tokenStorage          TokenStorage
	smsSender             SmsSender
	jwtCreator            JwtCreator
	tokenGenerator        TokenGenerator
	smsTemplates          map[string]string
	sendSmsRateLimiter    *rate.TotalRateLimiter
	verifyCodeRateLimiter *rate.TotalRateLimiter
	turnstileVerifier     turnstile.TurnStileVerifier
}

type SpaHandler struct {
	staticPath string
	indexPath  string
}

type Server struct {
	server *http.Server
	config ServerConfig
}

func (s *Server) ListenAndServe() error {
	if s.config.UseTls {
		return s.server.ListenAndServeTLS(s.config.TlsCertPath, s.config.TlsPrivKeyPath)
	}

	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// ServeHTTP inspects the URL path to locate a file within the static dir
// on the SPA handler. If a file is found, it will be served. If not, the
// file located at the index path on the SPA handler will be served. This
// is suitable behavior for serving an SPA (single page application).
// https://github.com/gorilla/mux?tab=readme-ov-file#serving-single-page-applications
func (h SpaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Join internally call path.Clean to prevent directory traversal
	path := filepath.Join(h.staticPath, r.URL.Path)
	// check whether a file exists or is a directory at the given path
	fi, err := os.Stat(path)
	if os.IsNotExist(err) || fi.IsDir() {
		// file does not exist or path is a directory, serve index.html
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}

	if err != nil {
		// if we got an error (that wasn't that the file doesn't exist) stating the
		// file, return a 500 internal server error and stop
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// otherwise, use http.FileServer to serve the static file
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func NewServer(state *ServerState, config ServerConfig) (*Server, error) {
	router := mux.NewRouter()

	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		if err != nil {
			slog.Error("failed to write health response", "error", err)
		}
	})

	// api to handle validating the phone number from within the Yivi app
	router.HandleFunc("/api/embedded/send", func(w http.ResponseWriter, r *http.Request) {
		handleEmbeddedIssuanceSendSms(state, w, r)
	})
	router.HandleFunc("/api/embedded/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerify(state, w, r)
	})

	// api to handle validating the phone number
	router.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		handleSendSms(state, w, r)
	})
	router.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerify(state, w, r)
	})
	spa := SpaHandler{staticPath: "../frontend/build", indexPath: "index.html"}
	router.PathPrefix("/").Handler(spa)

	addr := fmt.Sprintf("%v:%v", config.Host, config.Port)
	srv := &http.Server{
		Handler: router,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return &Server{
		server: srv,
		config: config,
	}, nil
}

// -----------------------------------------------------------------------------------

type EmbeddedIssuance_SendSmsPayload struct {
	PhoneNumber string `json:"phone"`
	Language    string `json:"language"`
}

func handleEmbeddedIssuanceSendSms(state *ServerState, w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Path
	ip := getIpAddressForRequest(r)
	logReceivedRequest(r, ip)

	defer closeRequestBody(r)

	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to read body of send-sms request", err, "endpoint", endpoint)
		return
	}

	var body EmbeddedIssuance_SendSmsPayload
	err = json.Unmarshal(bodyContent, &body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to parse json for body of send-sms request", err, "endpoint", endpoint)
		return
	}

	allow, timeout := state.sendSmsRateLimiter.Allow(ip, body.PhoneNumber)

	if !allow {
		respondWithRateLimitErr(w, endpoint, ip, body.PhoneNumber, timeout)
		return
	}

	token, err := state.tokenGenerator.GenerateToken()
	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to generate token", err, "endpoint", endpoint)
		return
	}

	err = state.tokenStorage.StoreToken(body.PhoneNumber, token)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to store token", err, "endpoint", endpoint)
		return
	}

	message, err := createSmsMessage(state.smsTemplates, token, body.Language)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to create sms", err, "endpoint", endpoint)
		return
	}

	slog.Info("sending sms",
		"endpoint", endpoint,
		"phone", logging.MaskPhone(body.PhoneNumber),
		"language", body.Language)
	err = state.smsSender.SendSms(body.PhoneNumber, message)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorSendingSms, "failed to send sms", err, "endpoint", endpoint)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// -----------------------------------------------------------------------------------

type SendSmsPayload struct {
	PhoneNumber string `json:"phone"`
	Language    string `json:"language"`
	Captcha     string `json:"captcha"`
}

func handleSendSms(state *ServerState, w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Path
	ip := getIpAddressForRequest(r)
	logReceivedRequest(r, ip)

	defer closeRequestBody(r)

	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to read body of send-sms request", err, "endpoint", endpoint)
		return
	}

	var body SendSmsPayload
	err = json.Unmarshal(bodyContent, &body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to parse json for body of send-sms request", err, "endpoint", endpoint)
		return
	}

	// Return an erro when the captcha is nil or empty
	if body.Captcha == "" {
		respondWithErr(w, http.StatusBadRequest, ErrorInvalidCaptcha, "captcha is required", fmt.Errorf("captcha is empty"), "endpoint", endpoint)
		return
	}

	if !state.turnstileVerifier.Verify(body.Captcha, ip) {
		respondWithErr(w, http.StatusBadRequest, ErrorInvalidCaptcha, "invalid captcha", fmt.Errorf("captcha validation failed"), "endpoint", endpoint)
		return
	}

	allow, timeout := state.sendSmsRateLimiter.Allow(ip, body.PhoneNumber)

	if !allow {
		respondWithRateLimitErr(w, endpoint, ip, body.PhoneNumber, timeout)
		return
	}

	token, err := state.tokenGenerator.GenerateToken()
	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to generate token", err, "endpoint", endpoint)
		return
	}

	err = state.tokenStorage.StoreToken(body.PhoneNumber, token)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to store token", err, "endpoint", endpoint)
		return
	}

	message, err := createSmsMessage(state.smsTemplates, token, body.Language)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to create sms", err, "endpoint", endpoint)
		return
	}

	slog.Info("sending sms",
		"endpoint", endpoint,
		"phone", logging.MaskPhone(body.PhoneNumber),
		"language", body.Language)
	err = state.smsSender.SendSms(body.PhoneNumber, message)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorSendingSms, "failed to send sms", err, "endpoint", endpoint)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// -----------------------------------------------------------------------------------

type VerifyPayload struct {
	PhoneNumber string `json:"phone"`
	Token       string `json:"token"`
}

type VerifyResponse struct {
	Jwt           string `json:"jwt"`
	IrmaServerURL string `json:"irma_server_url"`
}

func handleVerify(state *ServerState, w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Path
	ip := getIpAddressForRequest(r)
	logReceivedRequest(r, ip)

	defer closeRequestBody(r)

	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to read body of verify request", err, "endpoint", endpoint)
		return
	}

	var body VerifyPayload
	err = json.Unmarshal(bodyContent, &body)
	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to parse body as json", err, "endpoint", endpoint)
		return
	}

	allow, timeout := state.verifyCodeRateLimiter.Allow(ip, body.PhoneNumber)

	if !allow {
		respondWithRateLimitErr(w, endpoint, ip, body.PhoneNumber, timeout)
		return
	}

	expectedToken, err := state.tokenStorage.RetrieveToken(body.PhoneNumber)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorCannotValidateToken, "no active token request", err, "endpoint", endpoint, "phone", logging.MaskPhone(body.PhoneNumber))
		return
	}

	if body.Token != expectedToken {
		respondWithErr(w, http.StatusUnauthorized, ErrorCannotValidateToken, "token incorrect", nil, "endpoint", endpoint, "phone", logging.MaskPhone(body.PhoneNumber))
		return
	}

	jwt, err := state.jwtCreator.CreateJwt(body.PhoneNumber)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to create JWT", err, "endpoint", endpoint)
		return
	}

	responseMessage := VerifyResponse{
		Jwt:           jwt,
		IrmaServerURL: state.irmaServerURL,
	}

	payload, err := json.Marshal(responseMessage)
	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to marshal response message", err, "endpoint", endpoint)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(payload)
	if err != nil {
		slog.Error("failed to write body to http response", "error", err)
	}

	// can't really do anything about the error if it were to occur...
	err = state.tokenStorage.RemoveToken(body.PhoneNumber)
	if err != nil {
		slog.Error("error while removing token", "error", err, "phone", logging.MaskPhone(body.PhoneNumber))
	}
}

// -----------------------------------------------------------------------------------

// logReceivedRequest logs an incoming request with enough context
// (method, endpoint, client ip) to be useful for request tracing.
// It also includes the raw remote address and proxy headers so we can
// see how kubernetes internal routing presents client ips to the pod.
func logReceivedRequest(r *http.Request, ip string) {
	slog.Info("received request",
		"method", r.Method,
		"endpoint", r.URL.Path,
		"ip", ip,
		"remote_addr", r.RemoteAddr,
		"x_real_ip", r.Header.Get("X-Real-IP"),
		"x_forwarded_for", r.Header.Get("X-Forwarded-For"))
}

func getIpAddressForRequest(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return ip
}

func createSmsMessage(templates map[string]string, token, language string) (string, error) {
	if fmtString, ok := templates[language]; ok {
		return fmt.Sprintf(fmtString, token), nil
	}

	return "", fmt.Errorf("no template for language '%v'", language)
}

func closeRequestBody(r *http.Request) {
	if err := r.Body.Close(); err != nil {
		slog.Error("failed to close request body", "error", err)
	}
}

// respondWithRateLimitErr writes a 429 response with a Retry-After header and
// logs the relevant rate limiting context
func respondWithRateLimitErr(w http.ResponseWriter, endpoint, ip, phone string, timeout time.Duration) {
	// rounding so it doesn't show up weird on the client side
	roundedSecs := int(math.Round(timeout.Seconds()))
	w.Header().Set("Retry-After", fmt.Sprintf("%d", roundedSecs))
	respondWithErr(w, http.StatusTooManyRequests, ErrorRateLimit, "too many requests", nil,
		"endpoint", endpoint,
		"ip", ip,
		"phone", logging.MaskPhone(phone),
		"retry_after_seconds", roundedSecs)
}

func respondWithErr(w http.ResponseWriter, code int, responseBody string, logMsg string, e error, extras ...any) {
	args := []any{"status_code", code, "response_body", responseBody}
	if e != nil {
		args = append(args, "error", e)
	}
	args = append(args, extras...)
	// client errors (4xx) are expected operational events; only server
	// errors (5xx) should count towards the error-rate signal
	if code >= 500 {
		slog.Error(logMsg, args...)
	} else {
		slog.Warn(logMsg, args...)
	}
	w.WriteHeader(code)
	if _, err := w.Write([]byte(responseBody)); err != nil {
		slog.Error("failed to write body to http response", "error", err)
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"time"
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
	irmaServerURL   string
	tokenStorage    TokenStorage
	smsSender       SmsSender
	jwtCreator      JwtCreator
	tokenGenerator  TokenGenerator
	smsTemplates    map[string]string
	rateLimiter     *rate.TotalRateLimiter
	turnstileSecret string
}

type Server struct {
	server *http.Server
	config ServerConfig
}

func (s *Server) ListenAndServe() error {
	if s.config.UseTls {
		return s.server.ListenAndServeTLS(s.config.TlsCertPath, s.config.TlsPrivKeyPath)
	} else {
		return s.server.ListenAndServe()
	}
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func NewServer(state *ServerState, config ServerConfig) (*Server, error) {
	// static file server for the web part on the root
	fs := http.FileServer(http.Dir("../frontend/build"))

	mux := http.NewServeMux()

	mux.Handle("/", fs)

	// api to handle validating the phone number
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		handleSendSms(state, w, r)
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerify(state, w, r)
	})

	addr := fmt.Sprintf("%v:%v", config.Host, config.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return &Server{
		server: server,
		config: config,
	}, nil
}

// -----------------------------------------------------------------------------------

type SendSmsPayload struct {
	PhoneNumber string `json:"phone"`
	Language    string `json:"language"`
	Captcha     string `json:"captcha"`
}

func handleSendSms(state *ServerState, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to read body of send-sms request", err)
		return
	}

	var body SendSmsPayload
	err = json.Unmarshal(bodyContent, &body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to parse json for body of send-sms request", err)
		return
	}

	if !verifyTurnstile(state.turnstileSecret, body.Captcha, getIpAddressForRequest(r)) {
		respondWithErr(w, http.StatusBadRequest, ErrorInvalidCaptcha, "invalid captcha", fmt.Errorf("captcha validation failed"))
		return
	}

	ip := getIpAddressForRequest(r)

	allow, timeout := state.rateLimiter.Allow(ip, body.PhoneNumber)

	if !allow {
		// rounding so it doesn't show up weird on the client side
		roundedSecs := int(math.Round(timeout.Seconds()))
		w.Header().Set("Retry-After", fmt.Sprintf("%d", roundedSecs))
		respondWithErr(w, http.StatusTooManyRequests, ErrorRateLimit, "too many requests", err)
		return
	}

	token := state.tokenGenerator.GenerateToken()

	err = state.tokenStorage.StoreToken(body.PhoneNumber, token)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to store token", err)
		return
	}

	message, err := createSmsMessage(state.smsTemplates, body.PhoneNumber, token, body.Language)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to create sms", err)
		return
	}

	log.Info.Printf("Sending sms to %v: %v", body.PhoneNumber, message)
	err = state.smsSender.SendSms(body.PhoneNumber, message)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorSendingSms, "failed to send sms", err)
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
	defer r.Body.Close()
	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to read body of verify request", err)
		return
	}

	var body VerifyPayload
	err = json.Unmarshal(bodyContent, &body)
	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to parse body as json", err)
		return
	}

	expectedToken, err := state.tokenStorage.RetrieveToken(body.PhoneNumber)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorCannotValidateToken, "no active token request", err)
		return
	}

	if body.Token != expectedToken {
		respondWithErr(w, http.StatusUnauthorized, ErrorCannotValidateToken, "token incorrect", err)
		return
	}

	jwt, err := state.jwtCreator.CreateJwt(body.PhoneNumber)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to create JWT", err)
		return
	}

	responseMessage := VerifyResponse{
		Jwt:           jwt,
		IrmaServerURL: state.irmaServerURL,
	}

	payload, err := json.Marshal(responseMessage)
	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to marshal response message", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(payload)
	if err != nil {
		log.Error.Fatalf("failed to write body to http response: %v", err)
	}

	// can't really do anything about the error if it were to occur...
	err = state.tokenStorage.RemoveToken(body.PhoneNumber)
	if err != nil {
		log.Error.Printf("error while removing token: %v", err)
	}
}

// -----------------------------------------------------------------------------------

func getIpAddressForRequest(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return ip
}

func createSmsMessage(templates map[string]string, phone, token, language string) (string, error) {
	if fmtString, ok := templates[language]; ok {
		urlSuffix := fmt.Sprintf("%v:%v", phone, token)
		return fmt.Sprintf(fmtString, token, urlSuffix), nil
	} else {
		err := fmt.Errorf("no template for language '%v'", language)
		return "", err
	}
}

func respondWithErr(w http.ResponseWriter, code int, responseBody string, logMsg string, e error) {
	m := fmt.Sprintf("%v: %v", logMsg, e)
	log.Error.Printf("%s\n -> returning statuscode %d with message %v", m, code, responseBody)
	w.WriteHeader(code)
	if _, err := w.Write([]byte(responseBody)); err != nil {
		log.Error.Printf("failed to write body to http response: %v", err)
	}
}

func verifyTurnstile(secret, token, ip string) bool {
	values := url.Values{}
	values.Set("secret", secret)
	values.Set("response", token)
	if ip != "" {
		values.Set("remoteip", ip)
	}
	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", values)
	if err != nil {
		log.Error.Printf("turnstile request failed: %v", err)
		return false
	}
	defer resp.Body.Close()
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

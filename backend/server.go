package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// same error message bodies as the old Java code
const ErrorPhoneNumberFormat = "error:phone-number-format"
const ErrorRateLimit = "error:ratelimit"
const ErrorCannotValidateToken = "error:cannot-validate-token"
const ErrorAddressMalformed = "error:address-malformed"
const ErrorInternal = "error:internal"
const ErrorSendingSms = "error:sending-sms"

type ServerConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	UseTls         bool   `json:"use_tls,omitempty"`
	TlsPrivKeyPath string `json:"tls_priv_key_path,omitempty"`
	TlsCertPath    string `json:"tls_cert_path,omitempty"`
}

type ServerState struct {
	tokenRepo      TokenRepository
	smsSender      SmsSender
	jwtCreator     JwtCreator
	tokenGenerator TokenGenerator
	smsTemplates   map[string]string
	rateLimiter    *RateLimiter
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

func NewServer(state ServerState, config ServerConfig) (*Server, error) {
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
}

func handleSendSms(state ServerState, w http.ResponseWriter, r *http.Request) {
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

	ip := getIpAddressForRequest(r)

    allow, timeout := state.rateLimiter.Allow(ip, body.PhoneNumber)

    if !allow {
		respondWithErr(w, http.StatusTooManyRequests, ErrorRateLimit, "too many requests", err)
		w.Header().Add("Retry-After", fmt.Sprintf("%f", timeout.Seconds()))
    }

	token := state.tokenGenerator.GenerateToken()

	err = state.tokenRepo.StoreToken(body.PhoneNumber, token)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorInternal, "failed to store token", err)
		return
	}

	message, err := createSmsMessage(state.smsTemplates, body.Language, token)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to create sms", err)
		return
	}

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

func handleVerify(state ServerState, w http.ResponseWriter, r *http.Request) {
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

	expectedToken, err := state.tokenRepo.RetrieveToken(body.PhoneNumber)

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

	w.Write([]byte(jwt))
	w.WriteHeader(http.StatusOK)

	// can't really do anything about the error if it were to occur...
	err = state.tokenRepo.RemoveToken(body.PhoneNumber)
	if err != nil {
		ErrorLogger.Printf("error while removing token: %v", err)
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

func createSmsMessage(templates map[string]string, language string, token string) (string, error) {
	if fmtString, ok := templates[language]; ok {
		return fmt.Sprintf(fmtString, token), nil
	} else {
		err := fmt.Errorf("no template for language '%v'", language)
		return "", err
	}
}

func respondWithErr(w http.ResponseWriter, code int, responseBody string, log string, e error) {
	m := fmt.Sprintf(log+":", e)
	ErrorLogger.Printf("%s\n -> returning statuscode %d with message %v", m, code, responseBody)
	w.WriteHeader(code)
	w.Write([]byte(responseBody))
}

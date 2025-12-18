package main

import (
	"context"
	"encoding/json"
	"fmt"
	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	turnstile "go-sms-issuer/turnstile"
	"io"
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
	irmaServerURL     string
	tokenStorage      TokenStorage
	smsSender         SmsSender
	jwtCreator        JwtCreator
	tokenGenerator    TokenGenerator
	smsTemplates      map[string]string
	rateLimiter       *rate.TotalRateLimiter
	turnstileVerifier turnstile.TurnStileVerifier
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
			log.Error.Fatalf("failed to write body to http response: %v", err)
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
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error.Printf("failed to close request body: %v", err)
		}
	}()

	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to read body of send-sms request", err)
		return
	}

	var body EmbeddedIssuance_SendSmsPayload
	err = json.Unmarshal(bodyContent, &body)

	if err != nil {
		respondWithErr(w, http.StatusBadRequest, ErrorInternal, "failed to parse json for body of send-sms request", err)
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

	log.Info.Printf("sending sms")
	err = state.smsSender.SendSms(body.PhoneNumber, message)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorSendingSms, "failed to send sms", err)
		return
	}

	w.WriteHeader(http.StatusOK)

	err = r.Body.Close()
	if err != nil {
		log.Error.Printf("error while closing body: %v", err)
	}
}

// -----------------------------------------------------------------------------------

type SendSmsPayload struct {
	PhoneNumber string `json:"phone"`
	Language    string `json:"language"`
	Captcha     string `json:"captcha"`
}

func handleSendSms(state *ServerState, w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error.Printf("failed to close request body: %v", err)
		}
	}()

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

	// Return an erro when the captcha is nil or empty
	if body.Captcha == "" {
		respondWithErr(w, http.StatusBadRequest, ErrorInvalidCaptcha, "captcha is required", fmt.Errorf("captcha is empty"))
		return
	}

	if !state.turnstileVerifier.Verify(body.Captcha, getIpAddressForRequest(r)) {
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

	log.Info.Printf("sending sms")
	err = state.smsSender.SendSms(body.PhoneNumber, message)

	if err != nil {
		respondWithErr(w, http.StatusInternalServerError, ErrorSendingSms, "failed to send sms", err)
		return
	}

	w.WriteHeader(http.StatusOK)

	err = r.Body.Close()
	if err != nil {
		log.Error.Printf("error while closing body: %v", err)
	}
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
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error.Printf("failed to close request body: %v", err)
		}
	}()

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

	err = r.Body.Close()
	if err != nil {
		log.Error.Printf("error while closing body: %v", err)
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
	}

	return "", fmt.Errorf("no template for language '%v'", language)
}

func respondWithErr(w http.ResponseWriter, code int, responseBody string, logMsg string, e error) {
	m := fmt.Sprintf("%v: %v", logMsg, e)
	log.Error.Printf("%s\n -> returning statuscode %d with message %v", m, code, responseBody)
	w.WriteHeader(code)
	if _, err := w.Write([]byte(responseBody)); err != nil {
		log.Error.Printf("failed to write body to http response: %v", err)
	}
}

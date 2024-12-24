package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ServerConfig struct {
	Port int
	Host string
}

type Server struct {
	config    ServerConfig
	tokenRepo TokenRepository
	smsSender SmsSender
}

func StartServer(server Server) {
	fs := http.FileServer(http.Dir("./web/build"))
	http.Handle("/", fs)
	http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		handleSendSms(server, w, r)
	})
	http.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerify(server, w, r)
	})

	addr := fmt.Sprintf("%v:%v", server.config.Host, server.config.Port)
	err := http.ListenAndServe(addr, nil)

	if err != nil {
		ErrorLogger.Fatalf("failed to start server: %v", err)
	}
}

type VerifyPayload struct {
	PhoneNumber string `json:"phone"`
	Token       string `json:"token"`
}

func errorWithMessage(w http.ResponseWriter, code int, message string, e error) {
	m := fmt.Sprintf(message+":", e)
	ErrorLogger.Printf("%s\n -> returning statuscode %d", m, code)
	w.WriteHeader(code)
	w.Write([]byte(m))
}

func handleVerify(server Server, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		errorWithMessage(w, http.StatusBadRequest, "failed to read body of verify request", err)
		return
	}

	var body VerifyPayload
	err = json.Unmarshal(bodyContent, &body)
	if err != nil {
		errorWithMessage(w, http.StatusBadRequest, "failed to parse body as json", err)
		return
	}

	correctToken, err := server.tokenRepo.RetrieveToken(body.PhoneNumber)

	if err != nil {
		errorWithMessage(w, http.StatusBadRequest, "no active token request", err)
		return
	}

	if body.Token != correctToken {
		errorWithMessage(w, http.StatusUnauthorized, "token incorrect", err)
		return
	}

	jwt, err := CreateSessionRequestJWT(body.PhoneNumber)

	if err != nil {
		errorWithMessage(w, http.StatusInternalServerError, "failed to create JWT", err)
		return
	}

	w.Write([]byte(jwt))
	w.WriteHeader(http.StatusOK)

	// can't really do anything about the error if it were to occur...
	err = server.tokenRepo.RemoveToken(body.PhoneNumber)
	if err != nil {
		ErrorLogger.Printf("error while removing token: %v", err)
	}
}

type SendSmsPayload struct {
	PhoneNumber string `json:"phone"`
	Language    string `json:"language"`
}

func handleSendSms(server Server, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		errorWithMessage(w, http.StatusBadRequest, "failed to read body of send-sms request", err)
		return
	}

	var body SendSmsPayload
	err = json.Unmarshal(bodyContent, &body)

	if err != nil {
		errorWithMessage(w, http.StatusBadRequest, "failed to parse json for body of send-sms request", err)
		return
	}

	token := generateToken()

	err = server.tokenRepo.StoreToken(body.PhoneNumber, token)

	if err != nil {
		errorWithMessage(w, http.StatusInternalServerError, "failed to store token internally", err)
		return
	}

	err = server.smsSender.SendSms(body.PhoneNumber, body.Language, token)

	if err != nil {
		errorWithMessage(w, http.StatusInternalServerError, "failed to send sms", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func generateToken() string {
	return "123456"
}

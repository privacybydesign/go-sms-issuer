package main

import (
	"encoding/json"
	"io"
	"net/http"
)

func StartServer() {
	fs := http.FileServer(http.Dir("./web/build"))
	http.Handle("/", fs)
	http.HandleFunc("/send", handleSendSms)
    http.HandleFunc("/verify", handleVerify)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		ErrorLogger.Printf("failed to start server: %v", err)
	}
}

type VerifyPayload = struct {
	PhoneNumber string `json:"phone"`
	Token       string `json:"token"`
}

func handleVerify(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		ErrorLogger.Printf("failed to read body of verify request: %v", err)
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte("failed to read body of verify request"))
		return
	}

	var body VerifyPayload
	err = json.Unmarshal(bodyContent, &body)
	if err != nil {
		ErrorLogger.Printf("failed to parse body as json: %v", err)
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte("failed to parse body as json"))
		return
	}

	if body.PhoneNumber == "+31612345678" && body.Token == "123456" {
		InfoLogger.Printf("Correct phone number!")

        jwt, err := CreateSessionRequestJWT(body.PhoneNumber)

        if err != nil {
            InfoLogger.Printf("failed to create JWT: %v", jwt)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        w.Write([]byte(jwt))
		w.WriteHeader(http.StatusOK)
	} else {
		InfoLogger.Printf("Invalid phone number :-(")
        w.WriteHeader(http.StatusUnauthorized)
        w.Write([]byte("invalid phone number"))
	}

}

type SendSmsPayload = struct {
	PhoneNumber string `json:"phone"`
	Language    string `json:"language"`
}

func handleSendSms(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	bodyContent, err := io.ReadAll(r.Body)

	if err != nil {
		ErrorLogger.Printf("failed to read body of send sms request: %v", err)
		r.Response.StatusCode = 400
		return
	}

	var body SendSmsPayload
	err = json.Unmarshal(bodyContent, &body)

	if err != nil {
		ErrorLogger.Printf("failed to parse json for body of send sms request: %v", err)

		return
	}

	InfoLogger.Printf("sending sms to %v", body.PhoneNumber)
	w.WriteHeader(http.StatusOK)
}

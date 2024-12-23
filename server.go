package main

import (
	"encoding/json"
	"io"
	"net/http"
)

func startServer() {
	fs := http.FileServer(http.Dir("./web/build"))
	http.Handle("/", fs)
	http.HandleFunc("/send", handleSendSms)

    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        ErrorLogger.Printf("failed to start server: %v", err)
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
        ErrorLogger.Printf("failed to read body of send sms request")
        r.Response.StatusCode = 400
        return
    }

    var body SendSmsPayload
    err = json.Unmarshal(bodyContent, &body)

    if err != nil {
        ErrorLogger.Printf("failed to parse json for body of send sms request")
        r.Response.StatusCode = 400
        return
    }

    InfoLogger.Printf("sending sms to %v", body.PhoneNumber)
}

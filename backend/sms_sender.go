package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
    log "go-sms-issuer/logging"
)

type SmsSender interface {
	SendSms(phone, message string) error
}

type CmSmsSenderConfig struct {
	From         string `json:"from"`
	ApiEndpoint  string `json:"api_endpoint"`
	ProductToken string `json:"product_token"`
	Reference    string `json:"reference"`
	TimeoutMs    int64  `json:"timeout_ms"`
}

type CmSmsSender struct {
	CmSmsSenderConfig
}

func NewCmSmsSender(config CmSmsSenderConfig) (*CmSmsSender, error) {
	if strings.HasPrefix(config.ApiEndpoint, "https://") {
		return nil, errors.New("CM gateway API endpoint should use https")
	}

	if strings.HasSuffix(config.ApiEndpoint, "/") {
		return nil, errors.New("CM endpoint should not end with /")
	}

	return &CmSmsSender{config}, nil
}

// CM expects a phone number that starts with 00
func cmSanitizePhoneNumber(phone string) (string, error) {
	if strings.HasPrefix(phone, "+") {
		phone = fmt.Sprintf("00%s", phone[1:])
	} else if !strings.HasPrefix(phone, "00") {
		return "", errors.New("CM expects internationalized phone numbers")
	}
	return phone, nil
}

// Implements CM Gateway HTTP GET endpoint
// https://www.cm.com/en-en/app/docs/api/business-messaging-api/1.0/index#http-get
func (s *CmSmsSender) SendSms(phone, message string) error {
	phone, err := cmSanitizePhoneNumber(phone)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Add("producttoken", s.ProductToken)
	params.Add("from", s.From)
	params.Add("to", phone)
	params.Add("body", message)
	params.Add("reference", s.Reference)

	fullUrl := fmt.Sprintf("%s/gateway.ashx?%s", s.ApiEndpoint, params.Encode())

	client := &http.Client{Timeout: time.Duration(s.TimeoutMs) * time.Millisecond}

	resp, err := client.Get(fullUrl)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CM gateway returned status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	text := string(body)
	if text != "" {
		return fmt.Errorf("error response received from: %s", text)
	}

	return nil
}

type DummySmsSender struct{}

func (s *DummySmsSender) SendSms(phone, message string) error {
	log.Info.Printf("Sending sms to %v: %v", phone, message)
	return nil
}

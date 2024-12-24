package main

import (
	"bytes"
	"net/http"
	"net/url"
)

type SmsSender interface {
	SendSms(phone, message string) error
}

type CmSmsSenderConfig struct {
	From         string `json:"from"`
	ApiEndpoint  string `json:"api_endpoint"`
	ProductToken string `json:"product_toekn"`
	Reference    string `json:"reference"`
}

type CmSmsSender struct {
	CmSmsSenderConfig
}

func (s *CmSmsSender) SendSms(phone, message string) error {
	reqContent := url.Values{}
	reqContent.Add("phone", phone)
	reqContent.Add("message", message)

	// url encode the values
	reqBody := reqContent.Encode()

	smsServiceUrl := ""

	contentType := "application/x-www-form-urlencoded; charset=UTF-8"
	_, err := http.Post(smsServiceUrl, contentType, bytes.NewBuffer([]byte(reqBody)))

	if err != nil {
		return err
	}

	return nil
}

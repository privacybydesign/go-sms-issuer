package main

import (
	"bytes"
	"net/http"
	"net/url"
)

type SmsSender interface {
	SendSms(phone string, language string, token string) error
}

type CmSmsSender struct {
	From         string            `json:"from"`
	ApiEndpoint  string            `json:"api_endpoint"`
	ProductToken string            `json:"product_token"`
	Reference    string            `json:"reference"`
    // map from language to template
	SmsTemplates map[string]string `json:"sms_templates"`
}

func (s *CmSmsSender) SendSms(phone string, language string, token string) error {
	reqContent := url.Values{}
	reqContent.Add("phone", phone)
	reqContent.Add("token", token)

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

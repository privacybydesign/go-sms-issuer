package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/privacybydesign/irmago"
)

func customSessionRequestJWT() (string, error) {
	keyBytes, err := os.ReadFile(".secrets/private.pem")

	if err != nil {
		return "", err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)

	if err != nil {
		return "", err
	}

	issuer := "sidn-pbdf"
	schemeManager := "irma-demo"
	credential := fmt.Sprintf("%v.%v.mobilenumber", schemeManager, issuer)

	sessionRequest := sessionRequestJson{
		Context: "https://irma.app/ld/request/issuance/v2",
		Credentials: []map[string]any{{
			"credential": credential,
			"attributes": map[string]string{
				"mobilenumber": "0612345678",
			},
		}},
	}

	sessionRequestBody, err := json.MarshalIndent(sessionRequest, "", "    ")

	if err != nil {
		return "", err
	}


    token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims {
        "iat": time.Now().String(),
        "iss": "sms_issuer",
        "sub": "issue_request",
        "iprequest":  string(sessionRequestBody),
    })

    return token.SignedString(privateKey)
}

func createSessionRequestJWT() (string, error) {
	keyBytes, err := os.ReadFile(".secrets/private.pem")

	if err != nil {
		return "", err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)

	if err != nil {
		return "", err
	}

	issuanceRequest := irma.NewIssuanceRequest([]*irma.CredentialRequest{
		{
			CredentialTypeID: irma.NewCredentialTypeIdentifier("irma-demo.sidn-pbdf.mobilenumber"),
			Attributes: map[string]string{
				"mobilenumber": "0612345678",
			},
		},
	})

	req, err := irma.SignSessionRequest(
		issuanceRequest,
		jwt.GetSigningMethod(jwt.SigningMethodRS256.Alg()),
		privateKey,
		"sms_issuer",
	)

	return req, nil
}

func makeCreateSessionRequestWithJWT() error {
	url := "http://localhost:8080/session"

	jwt, err := createSessionRequestJWT()

	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jwt)))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "text/plain")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	fmt.Printf("response: %v\n", resp)

	defer resp.Body.Close()
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("response body: %v\n", string(result))

	return nil
}

type sessionRequestJson = struct {
	Context     string           `json:"@context"`
	Credentials []map[string]any `json:"credentials"`
}

func makeCreateSessionRequestWithAuthKey() error {
	url := "http://localhost:8080/session"

	issuer := "sidn-pbdf"
	schemeManager := "irma-demo"
	credential := fmt.Sprintf("%v.%v.mobilenumber", schemeManager, issuer)

	sessionRequest := sessionRequestJson{
		Context: "https://irma.app/ld/request/issuance/v2",
		Credentials: []map[string]any{{
			"credential": credential,
			"attributes": map[string]string{
				"mobilenumber": "0612345678",
			},
		}},
	}

	sessionRequestBody, err := json.MarshalIndent(sessionRequest, "", "    ")

	if err != nil {
		return err
	}

	fmt.Printf("Sending json: %v\n", string(sessionRequestBody))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(sessionRequestBody))

	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "123")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("succes: %v\n", string(body))

	return nil
}

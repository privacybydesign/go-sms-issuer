package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	err := makeCreateSessionRequest()
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}

func getPublicKey() {
	url := "https://is.staging.yivi.app/publickey"
	resp, err := http.Get(url)

	if err != nil {
		fmt.Printf("error making request: %v\n", err)
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		fmt.Printf("Error reading body: %v", err)
		return
	}

	bodyStr := string(body)

	fmt.Printf("Got response:\n%v\n", bodyStr)
}

type sessionRequestJson = struct {
	Context     string           `json:"@context"`
	Credentials []map[string]any `json:"credentials"`
}

func makeCreateSessionRequest() error {
    url := "http://localhost:8080/session"
	contentType := "application/json"

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

	sessionRequestBody, err := json.Marshal(sessionRequest)

	if err != nil {
		return err
	}

    fmt.Printf("Sending json: %v\n", string(sessionRequestBody))

	resp, err := http.Post(url, contentType, bytes.NewBuffer(sessionRequestBody))
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

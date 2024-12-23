package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
    startServer()
	err := makeCreateSessionRequestWithJWT()
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


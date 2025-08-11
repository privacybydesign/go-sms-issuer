package main

import "testing"

func TestCreatingJwt(t *testing.T) {
	path := "../test-secrets/priv.pem"
	issuerId := "sms_issuer"
	credential := "irma-demo.sidn-pbdf.mobilenumber"
	attribute := "mobilenumber"
	creator, err := NewIrmaJwtCreator(path, issuerId, credential, attribute)
	if err != nil {
		t.Fatalf("failed to instantiate jwt creator: %v", err)
	}

	jwt, err := creator.CreateJwt("+31612345678")
	if err != nil {
		t.Fatalf("failed to create jwt: %v", err)
	}

	if jwt == "" {
		t.Fatal("jwt is empty")
	}
}

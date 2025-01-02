package main

import "testing"

func TestCreatingJwt(t *testing.T) {
	path := "../local-secrets/sms-issuer/private.pem"
	issuerId := "sms_issuer"
	credential := "irma-demo.sidn-pbdf.mobilenumber"
	attribute := "mobilenumber"
	creator, err := NewDefaultJwtCreator(path, issuerId, credential, attribute)
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

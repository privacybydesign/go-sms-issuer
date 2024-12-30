package main

import (
	"time"

	"golang.org/x/exp/rand"
)

type TokenGenerator interface {
	GenerateToken() string
}

type RandomTokenGenerator struct{}

func (tg *RandomTokenGenerator) GenerateToken() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	rand.Seed(uint64(time.Now().UnixNano()))
	length := 6
	token := make([]byte, length)
	for i := range token {
		token[i] = charset[rand.Intn(len(charset))]
	}
	return string(token)
}

// for testing purposes it's useful to have a static token
// in production the RandomTokenGenerator should always be used
type StaticTokenGenerator struct {
	token string
}

func (tg *StaticTokenGenerator) GenerateToken() string {
	return tg.token
}

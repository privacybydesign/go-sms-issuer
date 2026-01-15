package main

import (
	log "go-sms-issuer/logging"
	"math/big"
	"math/rand"
	"sync"
	"time"

	crand "crypto/rand"
)

type TokenGenerator interface {
	GenerateToken() string
}

type RandomTokenGenerator struct {
	mu sync.Mutex
	r  *rand.Rand
}

func NewRandomTokenGenerator() *RandomTokenGenerator {
	return &RandomTokenGenerator{
		mu: sync.Mutex{},
		r:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (tg *RandomTokenGenerator) generateRandomInt(max int) int64 {
	num, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err == nil {
		return num.Int64()
	}
	log.Error.Printf("failed to generate cryptographically secure random number (max %v), falling back to pseudo random: %v", max, err)
	tg.mu.Lock()
	defer tg.mu.Unlock()

	return int64(tg.r.Intn(max))
}

func (tg *RandomTokenGenerator) GenerateToken() string {
	const (
		letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits  = "0123456789"
		length  = 6
	)
	token := make([]byte, length)

	// Anywhere between 2 and 6 digits
	numDigits := 2 + tg.generateRandomInt(4)

	// Add the digits first
	for i := range numDigits {
		token[i] = digits[tg.generateRandomInt(len(digits))]
	}

	// Fill remaining characters from full charset
	const charset = letters + digits
	for i := numDigits; i < length; i++ {
		token[i] = charset[tg.generateRandomInt(len(charset))]
	}

	// Shuffle to avoid predictable digit positions
	tg.r.Shuffle(len(token), func(i, j int) {
		token[i], token[j] = token[j], token[i]
	})

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

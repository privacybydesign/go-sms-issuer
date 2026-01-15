package main

import (
	"sync"
	"time"

	"math/rand"
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

func (tg *RandomTokenGenerator) GenerateToken() string {
	const (
		letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits  = "0123456789"
		length  = 6
	)
	token := make([]byte, length)

	tg.mu.Lock()
	defer tg.mu.Unlock()

	// Anywhere between 2 and 6 digits
	numDigits := 2 + tg.r.Intn(4)

	// Add the digits first
	for i := range numDigits {
		token[i] = digits[tg.r.Intn(len(digits))]
	}

	// 2) Fill remaining characters from full charset
	const charset = letters + digits
	for i := numDigits; i < length; i++ {
		token[i] = charset[tg.r.Intn(len(charset))]
	}

	// 3) Shuffle to avoid predictable digit positions
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

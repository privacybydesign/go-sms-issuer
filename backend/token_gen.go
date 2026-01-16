package main

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
)

type TokenGenerator interface {
	GenerateToken() (string, error)
}

type RandomTokenGenerator struct {
}

func NewRandomTokenGenerator() *RandomTokenGenerator {
	return &RandomTokenGenerator{}
}

func generateRandomNumber(max int) (int, error) {
	num, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random number: %w", err)
	}
	return int(num.Int64()), nil
}

// Cryptographically random shuffle
func cryptoShuffle[T any](s []T) error {
	for i := len(s) - 1; i > 0; i-- {
		// random j in [0, i]
		j, err := generateRandomNumber(i + 1)
		if err != nil {
			return err
		}
		s[i], s[j] = s[j], s[i]
	}
	return nil
}

func (tg *RandomTokenGenerator) GenerateToken() (string, error) {
	const (
		letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits  = "0123456789"
		length  = 6
	)
	token := make([]byte, length)

	// Anywhere between 2 and 6 digits
	numDigits, err := generateRandomNumber(4)
	if err != nil {
		return "", err
	}
	numDigits += 2

	// Add the digits first
	for i := range numDigits {
		r, err := generateRandomNumber(len(digits))
		if err != nil {
			return "", err
		}
		token[i] = digits[r]
	}

	// Fill remaining characters from full charset
	const charset = letters + digits
	for i := numDigits; i < length; i++ {
		r, err := generateRandomNumber(len(digits))
		if err != nil {
			return "", err
		}
		token[i] = charset[r]
	}

	// Shuffle to avoid predictable digit positions
	err = cryptoShuffle(token)
	return string(token), err
}

// for testing purposes it's useful to have a static token
// in production the RandomTokenGenerator should always be used
type StaticTokenGenerator struct {
	token string
}

func (tg *StaticTokenGenerator) GenerateToken() (string, error) {
	return tg.token, nil
}

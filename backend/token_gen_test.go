package main

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

func isAllowedChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func TestGenerateToken_Properties(t *testing.T) {
	tg := NewRandomTokenGenerator()

	const iterations = 1000
	for range iterations {
		token, err := tg.GenerateToken()
		require.NoError(t, err)

		// Check each code is 6 characters
		if len(token) != 6 {
			t.Fatalf("expected length 6, got %d: %q", len(token), token)
		}

		// Check if they're all part of the allowed charset
		for _, r := range token {
			if !isAllowedChar(r) {
				t.Fatalf("token contains invalid character %q in %q", string(r), token)
			}
			// Ensure uppercase only
			if unicode.IsLetter(r) && !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ", r) {
				t.Fatalf("token contains non-uppercase letter %q in %q", string(r), token)
			}
		}

		// Assert at least 2 digits
		d := countDigits(token)
		if d < 2 {
			t.Fatalf("expected at least 2 digits, got %d in %q", d, token)
		}
	}
}

func TestGenerateToken_BasicUniquenessSanity(t *testing.T) {
	// This is a sanity check, NOT a cryptographic test.
	// It can theoretically fail by chance, but with these params it should be extremely unlikely.
	tg := NewRandomTokenGenerator()

	const n = 500
	seen := make(map[string]struct{}, n)

	for range n {
		token, err := tg.GenerateToken()
		require.NoError(t, err)
		seen[token] = struct{}{}
	}

	// If randomness is totally broken, you'd see a tiny number of uniques.
	// With a proper generator, you should get almost all unique values here.
	if len(seen) < n-5 {
		t.Fatalf("expected near-unique tokens; got %d unique out of %d", len(seen), n)
	}
}

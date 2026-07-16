// Package pow implements a Hashcash-style proof-of-work challenge.
//
// The embedded issuance endpoint (/api/embedded/send) is reached directly by
// the Yivi app without a Turnstile captcha. To keep automated bulk requests
// expensive, the client must solve a proof-of-work challenge before an SMS is
// sent: it finds a nonce whose SHA-256 hash of "challenge:nonce" starts with a
// configurable number of zero bits.
//
// Challenges are stateless: the issuer signs each one with an HMAC over its
// fields, so it can verify a returned solution without storing anything. A
// short expiry plus single-use tracking (see SeenStore) stops a solved
// challenge from being replayed for more than one SMS.
package pow

import (
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Challenge is handed to the client. The client must find a nonce whose hash
// meets Difficulty before the issuer will send an SMS. Expiry is a Unix
// timestamp (seconds); Signature is the issuer's HMAC over the other fields so
// the challenge can be verified statelessly.
type Challenge struct {
	Challenge  string `json:"challenge"`
	Difficulty int    `json:"difficulty"`
	Expiry     int64  `json:"expiry"`
	Signature  string `json:"signature"`
}

// Solution is what the client submits with its send request: the original
// challenge fields plus the nonce it found.
type Solution struct {
	Challenge  string `json:"challenge"`
	Difficulty int    `json:"difficulty"`
	Expiry     int64  `json:"expiry"`
	Signature  string `json:"signature"`
	Nonce      int    `json:"nonce"`
}

// Verifier issues and verifies proof-of-work challenges.
type Verifier interface {
	// Enabled reports whether proof of work is enforced. When false, callers
	// should skip both challenge issuance and verification.
	Enabled() bool
	// NewChallenge issues a fresh, signed challenge.
	NewChallenge() (Challenge, error)
	// Verify reports nil when sol is a valid, unexpired, unused solution.
	Verify(sol Solution) error
}

var (
	ErrDisabled        = errors.New("proof of work is disabled")
	ErrBadSignature    = errors.New("challenge signature is invalid")
	ErrExpired         = errors.New("challenge has expired")
	ErrWrongDifficulty = errors.New("challenge difficulty does not match server policy")
	ErrUnsolved        = errors.New("nonce does not solve the challenge")
	ErrReplayed        = errors.New("challenge has already been used")
)

// DisabledVerifier is the no-op verifier used when proof of work is turned off.
// Existing config files that predate proof of work fall back to this, so the
// embedded endpoint keeps working unchanged.
type DisabledVerifier struct{}

func (DisabledVerifier) Enabled() bool { return false }

func (DisabledVerifier) NewChallenge() (Challenge, error) {
	return Challenge{}, ErrDisabled
}

func (DisabledVerifier) Verify(Solution) error { return nil }

// SeenStore records which challenges have already been spent so a single
// solution cannot be replayed for more than one SMS. MarkSeen must return true
// only the first time key is passed within its ttl; later calls return false.
type SeenStore interface {
	MarkSeen(key string, ttl time.Duration) (bool, error)
}

// HmacVerifier signs challenges with an HMAC so they can be verified without
// server-side state, and tracks spent challenges through a SeenStore.
type HmacVerifier struct {
	secret     []byte
	difficulty int
	ttl        time.Duration
	seen       SeenStore
	// now is injectable so tests can control time; defaults to time.Now.
	now func() time.Time
}

// NewHmacVerifier builds a verifier. secret must be non-empty, difficulty must
// be positive and ttl must be positive.
func NewHmacVerifier(secret []byte, difficulty int, ttl time.Duration, seen SeenStore) (*HmacVerifier, error) {
	if len(secret) == 0 {
		return nil, errors.New("proof-of-work secret must not be empty")
	}
	if difficulty <= 0 {
		return nil, fmt.Errorf("proof-of-work difficulty must be positive, got %d", difficulty)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("proof-of-work ttl must be positive, got %v", ttl)
	}
	if seen == nil {
		return nil, errors.New("proof-of-work seen store must not be nil")
	}
	return &HmacVerifier{
		secret:     secret,
		difficulty: difficulty,
		ttl:        ttl,
		seen:       seen,
		now:        time.Now,
	}, nil
}

func (v *HmacVerifier) Enabled() bool { return true }

func (v *HmacVerifier) NewChallenge() (Challenge, error) {
	raw := make([]byte, 16)
	if _, err := crand.Read(raw); err != nil {
		return Challenge{}, fmt.Errorf("failed to generate challenge: %w", err)
	}
	c := Challenge{
		Challenge:  hex.EncodeToString(raw),
		Difficulty: v.difficulty,
		Expiry:     v.now().Add(v.ttl).Unix(),
	}
	c.Signature = v.sign(c)
	return c, nil
}

func (v *HmacVerifier) Verify(sol Solution) error {
	challenge := Challenge{
		Challenge:  sol.Challenge,
		Difficulty: sol.Difficulty,
		Expiry:     sol.Expiry,
		Signature:  sol.Signature,
	}

	// 1. The challenge must be one we issued (valid HMAC).
	expected := v.sign(challenge)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(sol.Signature)) != 1 {
		return ErrBadSignature
	}

	// 2. It must not have expired.
	if v.now().Unix() > sol.Expiry {
		return ErrExpired
	}

	// 3. Its difficulty must match the current server policy, so a client
	// cannot replay an old, easier challenge after the difficulty is raised.
	if sol.Difficulty != v.difficulty {
		return ErrWrongDifficulty
	}

	// 4. The nonce must actually solve the challenge.
	if !MeetsDifficulty(sol.Challenge, sol.Nonce, sol.Difficulty) {
		return ErrUnsolved
	}

	// 5. It must not have been used before (single-use within its lifetime).
	// The remaining lifetime bounds how long we must remember it.
	remaining := time.Unix(sol.Expiry, 0).Sub(v.now())
	fresh, err := v.seen.MarkSeen(sol.Signature, remaining)
	if err != nil {
		return fmt.Errorf("failed to record challenge use: %w", err)
	}
	if !fresh {
		return ErrReplayed
	}

	return nil
}

// sign returns the hex HMAC-SHA256 over the challenge's fields.
func (v *HmacVerifier) sign(c Challenge) string {
	mac := hmac.New(sha256.New, v.secret)
	// Fixed, unambiguous field order separated by ':'.
	mac.Write([]byte(c.Challenge))
	mac.Write([]byte(":"))
	mac.Write([]byte(strconv.Itoa(c.Difficulty)))
	mac.Write([]byte(":"))
	mac.Write([]byte(strconv.FormatInt(c.Expiry, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// MeetsDifficulty reports whether SHA-256("challenge:nonce") starts with at
// least difficulty zero bits.
func MeetsDifficulty(challenge string, nonce, difficulty int) bool {
	digest := sha256.Sum256([]byte(challenge + ":" + strconv.Itoa(nonce)))
	return leadingZeroBits(digest[:]) >= difficulty
}

// leadingZeroBits counts the number of leading zero bits in b.
func leadingZeroBits(b []byte) int {
	count := 0
	for _, x := range b {
		if x == 0 {
			count += 8
			continue
		}
		for mask := byte(0x80); mask != 0; mask >>= 1 {
			if x&mask != 0 {
				return count
			}
			count++
		}
		break
	}
	return count
}

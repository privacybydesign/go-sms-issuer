// Package altcha gates the embedded issuance endpoint (/api/embedded/send)
// with an ALTCHA v2 proof of work.
//
// That endpoint is reached directly by the Yivi app and, unlike the public web
// frontend, has no Turnstile captcha in front of it. To make automated bulk
// requests expensive, the client must solve a short proof-of-work challenge
// before an SMS is sent: it brute-forces a counter whose PBKDF2/SHA-256 key
// starts with a server-chosen prefix.
//
// Challenges are stateless: the issuer signs each one with an HMAC over its
// parameters (algorithm, cost, salt, nonce, key prefix and expiry), so a
// returned solution can be verified without storing the challenge. A short
// expiry plus single-use tracking (see SeenStore) stops a solved challenge
// from being replayed for more than one SMS.
//
// The ALTCHA protocol and its Go/Dart bindings only interoperate on v2 with a
// pinned key-derivation function, so the algorithm is fixed to PBKDF2/SHA-256
// here and any challenge naming a different one is rejected. See
// privacybydesign/irmamobile#667 for the wider plan this is part of.
package altcha

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	lib "github.com/altcha-org/altcha-lib-go/v2"
)

// PinnedAlgorithm is the only key-derivation function this issuer will create
// or accept. It is baked into the signed challenge, so a client cannot
// downgrade it to a cheaper algorithm.
const PinnedAlgorithm = "PBKDF2/SHA-256"

// EnforcementState controls how strictly the embedded endpoint applies the
// proof of work. It maps directly onto the staged rollout in the plan.
type EnforcementState int

const (
	// Disabled: no challenge is handed out (the endpoint 404s) and sends are
	// accepted without a solution. This is the default and preserves the
	// captcha-free behaviour existing clients rely on.
	Disabled EnforcementState = iota
	// Monitor: challenges are handed out and solutions are verified, but a
	// send that lacks or fails one is still accepted and logged. This is the
	// measured grace window used to watch old apps roll over before enforcing.
	Monitor
	// Enforced: a send is rejected unless it carries a valid, unused solution.
	Enforced
)

// ParseEnforcementState maps a config string onto an EnforcementState. An
// empty string means Disabled so configs that predate ALTCHA keep working.
func ParseEnforcementState(s string) (EnforcementState, error) {
	switch s {
	case "", "disabled":
		return Disabled, nil
	case "monitor":
		return Monitor, nil
	case "enforced":
		return Enforced, nil
	default:
		return Disabled, fmt.Errorf("invalid altcha_backend %q (want disabled, monitor or enforced)", s)
	}
}

// Verifier issues and verifies ALTCHA challenges.
type Verifier interface {
	// State reports the current enforcement state.
	State() EnforcementState
	// Enabled reports whether challenges are handed out at all (Monitor or
	// Enforced). When false the challenge endpoint should 404.
	Enabled() bool
	// NewChallenge issues a fresh, signed challenge.
	NewChallenge() (lib.Challenge, error)
	// Verify reports nil when payload is a valid, unexpired, unused solution.
	// payload is the base64-encoded JSON ALTCHA payload submitted by the client.
	Verify(payload string) error
}

var (
	ErrDisabled       = errors.New("altcha is disabled")
	ErrMalformed      = errors.New("altcha payload is malformed")
	ErrWrongAlgorithm = errors.New("altcha challenge uses an unexpected algorithm")
	ErrExpired        = errors.New("altcha challenge has expired")
	ErrBadSignature   = errors.New("altcha challenge signature is invalid")
	ErrUnsolved       = errors.New("altcha solution is invalid")
	ErrReplayed       = errors.New("altcha challenge has already been used")
)

// DisabledVerifier is the no-op verifier used when the proof of work is turned
// off. Existing config files that predate ALTCHA fall back to this, so the
// embedded endpoint keeps working unchanged.
type DisabledVerifier struct{}

func (DisabledVerifier) State() EnforcementState              { return Disabled }
func (DisabledVerifier) Enabled() bool                        { return false }
func (DisabledVerifier) NewChallenge() (lib.Challenge, error) { return lib.Challenge{}, ErrDisabled }
func (DisabledVerifier) Verify(string) error                  { return ErrDisabled }

// SeenStore records which challenges have already been spent so a single
// solution cannot be replayed for more than one SMS. MarkSeen must return true
// only the first time key is passed within its ttl; later calls return false.
// ALTCHA verifies a challenge's signature and expiry but does not track replay
// itself, so this stays our responsibility.
type SeenStore interface {
	MarkSeen(key string, ttl time.Duration) (bool, error)
}

// HmacVerifier signs challenges with an HMAC (via the ALTCHA library) so they
// can be verified without server-side state, and tracks spent challenges
// through a SeenStore.
type HmacVerifier struct {
	state           EnforcementState
	secret          string
	cost            int
	keyPrefixLength int
	ttl             time.Duration
	seen            SeenStore
	deriveKey       lib.DeriveKeyFunc
	// now is injectable so tests can control the SeenStore ttl; defaults to
	// time.Now. The library uses its own clock for expiry.
	now func() time.Time
}

// NewHmacVerifier builds a verifier. secret must be non-empty; cost and
// keyPrefixLength must be positive; ttl must be positive. state must be Monitor
// or Enforced (use DisabledVerifier for Disabled).
func NewHmacVerifier(state EnforcementState, secret string, cost, keyPrefixLength int, ttl time.Duration, seen SeenStore) (*HmacVerifier, error) {
	if state != Monitor && state != Enforced {
		return nil, fmt.Errorf("altcha verifier state must be monitor or enforced, got %d", state)
	}
	if secret == "" {
		return nil, errors.New("altcha secret must not be empty")
	}
	if cost <= 0 {
		return nil, fmt.Errorf("altcha cost must be positive, got %d", cost)
	}
	if keyPrefixLength <= 0 {
		return nil, fmt.Errorf("altcha key_prefix_length must be positive, got %d", keyPrefixLength)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("altcha ttl must be positive, got %v", ttl)
	}
	if seen == nil {
		return nil, errors.New("altcha seen store must not be nil")
	}
	return &HmacVerifier{
		state:           state,
		secret:          secret,
		cost:            cost,
		keyPrefixLength: keyPrefixLength,
		ttl:             ttl,
		seen:            seen,
		deriveKey:       lib.DeriveKeyPBKDF2(),
		now:             time.Now,
	}, nil
}

func (v *HmacVerifier) State() EnforcementState { return v.state }

func (v *HmacVerifier) Enabled() bool { return true }

func (v *HmacVerifier) NewChallenge() (lib.Challenge, error) {
	expiresAt := v.now().Add(v.ttl)
	// No Counter is passed, so the library leaves the key prefix as the target
	// the client must match: a keyPrefixLength-byte run of zero bytes. The
	// client brute-forces a counter whose derived key starts with it. Difficulty
	// is cost (PBKDF2 iterations per attempt) times the expected number of
	// attempts (256^keyPrefixLength); both are server-controlled, so difficulty
	// can be raised without a client update.
	return lib.CreateChallenge(lib.CreateChallengeOptions{
		Algorithm:           PinnedAlgorithm,
		DeriveKey:           v.deriveKey,
		HMACSignatureSecret: v.secret,
		Cost:                v.cost,
		KeyLength:           32,
		KeyPrefix:           zeroPrefix(v.keyPrefixLength),
		ExpiresAt:           &expiresAt,
	})
}

func (v *HmacVerifier) Verify(payload string) error {
	if payload == "" {
		return ErrMalformed
	}

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	var parsed lib.Payload
	if err := json.Unmarshal(decoded, &parsed); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	// The algorithm lives inside the signed parameters, so a valid signature
	// already guarantees it is ours; checking it explicitly rejects malformed
	// payloads early and documents the pin.
	if parsed.Challenge.Parameters.Algorithm != PinnedAlgorithm {
		return ErrWrongAlgorithm
	}

	result, err := lib.VerifySolution(lib.VerifySolutionOptions{
		Challenge:           parsed.Challenge,
		Solution:            parsed.Solution,
		DeriveKey:           v.deriveKey,
		HMACSignatureSecret: v.secret,
	})
	if err != nil {
		return fmt.Errorf("altcha verification failed: %w", err)
	}

	if result.Expired {
		return ErrExpired
	}
	if result.InvalidSignature != nil && *result.InvalidSignature {
		return ErrBadSignature
	}
	if !result.Verified {
		return ErrUnsolved
	}

	// Single-use within its lifetime. Key on the challenge signature (unique
	// per issued challenge) and bound how long we must remember it by the
	// remaining lifetime. Only mark a cryptographically valid solution as seen,
	// so an attacker cannot burn arbitrary keys with junk payloads.
	remaining := v.remainingLifetime(parsed.Challenge.Parameters.ExpiresAt)
	fresh, err := v.seen.MarkSeen(parsed.Challenge.Signature, remaining)
	if err != nil {
		return fmt.Errorf("failed to record challenge use: %w", err)
	}
	if !fresh {
		return ErrReplayed
	}

	return nil
}

// remainingLifetime returns how long a challenge with the given Unix expiry has
// left. A challenge without an expiry (unexpected, since we always set one)
// falls back to the configured ttl.
func (v *HmacVerifier) remainingLifetime(expiresAt int64) time.Duration {
	if expiresAt <= 0 {
		return v.ttl
	}
	return time.Unix(expiresAt, 0).Sub(v.now())
}

// zeroPrefix returns the hex encoding of n zero bytes, e.g. n=2 -> "0000".
// This is the key prefix a client's derived key must start with.
func zeroPrefix(n int) string {
	const hexZeroByte = "00"
	out := make([]byte, 0, n*len(hexZeroByte))
	for range n {
		out = append(out, hexZeroByte...)
	}
	return string(out)
}

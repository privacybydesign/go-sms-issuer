package altcha

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	lib "github.com/altcha-org/altcha-lib-go/v2"
	"github.com/stretchr/testify/require"
)

// solve fetches a fresh challenge from v, solves it with the reference library
// solver and returns the base64 payload a client would submit.
func solve(t *testing.T, v *HmacVerifier) string {
	t.Helper()
	challenge, err := v.NewChallenge()
	require.NoError(t, err)

	solution, err := lib.SolveChallenge(lib.SolveChallengeOptions{
		Challenge: challenge,
		DeriveKey: lib.DeriveKeyPBKDF2(),
	})
	require.NoError(t, err)
	require.NotNil(t, solution)

	raw, err := json.Marshal(lib.Payload{Challenge: challenge, Solution: *solution})
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func newTestVerifier(t *testing.T, store SeenStore) *HmacVerifier {
	t.Helper()
	v, err := NewHmacVerifier(Enforced, "test-secret", 50, 1, time.Minute, store)
	require.NoError(t, err)
	return v
}

func TestParseEnforcementState(t *testing.T) {
	cases := map[string]EnforcementState{"": Disabled, "disabled": Disabled, "monitor": Monitor, "enforced": Enforced}
	for in, want := range cases {
		got, err := ParseEnforcementState(in)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	_, err := ParseEnforcementState("nonsense")
	require.Error(t, err)
}

func TestNewHmacVerifierValidatesArgs(t *testing.T) {
	store := NewInMemorySeenStore()
	_, err := NewHmacVerifier(Disabled, "s", 1, 1, time.Minute, store)
	require.Error(t, err, "Disabled is not a valid concrete-verifier state")
	_, err = NewHmacVerifier(Enforced, "", 1, 1, time.Minute, store)
	require.Error(t, err)
	_, err = NewHmacVerifier(Enforced, "s", 0, 1, time.Minute, store)
	require.Error(t, err)
	_, err = NewHmacVerifier(Enforced, "s", 1, 0, time.Minute, store)
	require.Error(t, err)
	_, err = NewHmacVerifier(Enforced, "s", 1, 1, 0, store)
	require.Error(t, err)
	_, err = NewHmacVerifier(Enforced, "s", 1, 1, time.Minute, nil)
	require.Error(t, err)
}

func TestChallengeUsesPinnedAlgorithm(t *testing.T) {
	v := newTestVerifier(t, NewInMemorySeenStore())
	challenge, err := v.NewChallenge()
	require.NoError(t, err)
	require.Equal(t, PinnedAlgorithm, challenge.Parameters.Algorithm)
	require.NotEmpty(t, challenge.Signature)
	require.Positive(t, challenge.Parameters.ExpiresAt)
}

func TestVerifyRoundTrip(t *testing.T) {
	v := newTestVerifier(t, NewInMemorySeenStore())
	require.NoError(t, v.Verify(solve(t, v)))
}

func TestVerifyRejectsReplay(t *testing.T) {
	v := newTestVerifier(t, NewInMemorySeenStore())
	payload := solve(t, v)
	require.NoError(t, v.Verify(payload))
	require.ErrorIs(t, v.Verify(payload), ErrReplayed)
}

func TestVerifyRejectsMalformedPayload(t *testing.T) {
	v := newTestVerifier(t, NewInMemorySeenStore())
	require.ErrorIs(t, v.Verify(""), ErrMalformed)
	require.ErrorIs(t, v.Verify("not-base64!!"), ErrMalformed)
	require.ErrorIs(t, v.Verify(base64.StdEncoding.EncodeToString([]byte("not json"))), ErrMalformed)
}

func TestVerifyRejectsWrongAlgorithm(t *testing.T) {
	v := newTestVerifier(t, NewInMemorySeenStore())
	challenge, err := v.NewChallenge()
	require.NoError(t, err)
	challenge.Parameters.Algorithm = "PBKDF2/SHA-512"
	raw, err := json.Marshal(lib.Payload{Challenge: challenge, Solution: lib.Solution{}})
	require.NoError(t, err)
	require.ErrorIs(t, v.Verify(base64.StdEncoding.EncodeToString(raw)), ErrWrongAlgorithm)
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	v := newTestVerifier(t, NewInMemorySeenStore())
	// A challenge signed with a different secret must not verify.
	other, err := NewHmacVerifier(Enforced, "different-secret", 50, 1, time.Minute, NewInMemorySeenStore())
	require.NoError(t, err)
	require.ErrorIs(t, v.Verify(solve(t, other)), ErrBadSignature)
}

func TestVerifyRejectsExpiredChallenge(t *testing.T) {
	// A verifier whose challenges are already expired must reject them.
	v, err := NewHmacVerifier(Enforced, "test-secret", 50, 1, time.Minute, NewInMemorySeenStore())
	require.NoError(t, err)
	// Freeze "now" in the past so the challenge's expiry has passed by the time
	// it is verified.
	v.now = func() time.Time { return time.Now().Add(-2 * time.Minute) }
	payload := solve(t, v)
	require.ErrorIs(t, v.Verify(payload), ErrExpired)
}

// failingStore is a SeenStore that always errors, to check fail-closed behaviour.
type failingStore struct{}

func (failingStore) MarkSeen(string, time.Duration) (bool, error) {
	return false, errors.New("seen store unavailable")
}

func TestVerifyFailsClosedOnStoreError(t *testing.T) {
	v := newTestVerifier(t, failingStore{})
	err := v.Verify(solve(t, v))
	require.Error(t, err, "a SeenStore error must propagate so the handler rejects the send")
}

package pow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// solve brute-forces a nonce for the given challenge at its difficulty, the
// same way the client does.
func solve(t *testing.T, c Challenge) Solution {
	t.Helper()
	for nonce := 0; ; nonce++ {
		if MeetsDifficulty(c.Challenge, nonce, c.Difficulty) {
			return Solution{
				Challenge:  c.Challenge,
				Difficulty: c.Difficulty,
				Expiry:     c.Expiry,
				Signature:  c.Signature,
				Nonce:      nonce,
			}
		}
	}
}

func newVerifier(t *testing.T, difficulty int) *HmacVerifier {
	t.Helper()
	v, err := NewHmacVerifier([]byte("test-secret"), difficulty, time.Minute, NewInMemorySeenStore())
	require.NoError(t, err)
	return v
}

func TestLeadingZeroBits(t *testing.T) {
	require.Equal(t, 16, leadingZeroBits([]byte{0x00, 0x00, 0xff}))
	require.Equal(t, 4, leadingZeroBits([]byte{0x0f}))
	require.Equal(t, 7, leadingZeroBits([]byte{0x01}))
	require.Equal(t, 0, leadingZeroBits([]byte{0x80}))
	require.Equal(t, 12, leadingZeroBits([]byte{0x00, 0x0f}))
}

func TestNewHmacVerifierRejectsBadConfig(t *testing.T) {
	_, err := NewHmacVerifier(nil, 20, time.Minute, NewInMemorySeenStore())
	require.Error(t, err)
	_, err = NewHmacVerifier([]byte("s"), 0, time.Minute, NewInMemorySeenStore())
	require.Error(t, err)
	_, err = NewHmacVerifier([]byte("s"), 20, 0, NewInMemorySeenStore())
	require.Error(t, err)
	_, err = NewHmacVerifier([]byte("s"), 20, time.Minute, nil)
	require.Error(t, err)
}

func TestVerifyAcceptsValidSolution(t *testing.T) {
	v := newVerifier(t, 8)
	c, err := v.NewChallenge()
	require.NoError(t, err)
	require.Equal(t, 8, c.Difficulty)

	require.NoError(t, v.Verify(solve(t, c)))
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	v := newVerifier(t, 8)
	c, err := v.NewChallenge()
	require.NoError(t, err)
	sol := solve(t, c)
	sol.Signature = "deadbeef"
	require.ErrorIs(t, v.Verify(sol), ErrBadSignature)
}

func TestVerifyRejectsForgedChallenge(t *testing.T) {
	v := newVerifier(t, 8)
	// A challenge we never issued, signed by nobody.
	forged := Challenge{Challenge: "forged", Difficulty: 8, Expiry: time.Now().Add(time.Minute).Unix()}
	require.ErrorIs(t, v.Verify(solve(t, forged)), ErrBadSignature)
}

func TestVerifyRejectsWrongNonce(t *testing.T) {
	v := newVerifier(t, 12)
	c, err := v.NewChallenge()
	require.NoError(t, err)
	sol := solve(t, c)
	sol.Nonce++ // almost certainly no longer solves difficulty 12
	require.ErrorIs(t, v.Verify(sol), ErrUnsolved)
}

func TestVerifyRejectsExpiredChallenge(t *testing.T) {
	v := newVerifier(t, 8)
	base := time.Now()
	v.now = func() time.Time { return base }
	c, err := v.NewChallenge()
	require.NoError(t, err)
	sol := solve(t, c)

	// Jump past the challenge's expiry.
	v.now = func() time.Time { return time.Unix(c.Expiry, 0).Add(time.Second) }
	require.ErrorIs(t, v.Verify(sol), ErrExpired)
}

func TestVerifyRejectsMismatchedDifficulty(t *testing.T) {
	v := newVerifier(t, 8)
	c, err := v.NewChallenge()
	require.NoError(t, err)
	sol := solve(t, c)

	// Server difficulty was raised after the challenge was issued.
	v.difficulty = 9
	require.ErrorIs(t, v.Verify(sol), ErrWrongDifficulty)
}

func TestVerifyRejectsReplay(t *testing.T) {
	v := newVerifier(t, 8)
	c, err := v.NewChallenge()
	require.NoError(t, err)
	sol := solve(t, c)

	require.NoError(t, v.Verify(sol))
	require.ErrorIs(t, v.Verify(sol), ErrReplayed)
}

func TestDisabledVerifier(t *testing.T) {
	var v Verifier = DisabledVerifier{}
	require.False(t, v.Enabled())
	require.NoError(t, v.Verify(Solution{}))
	_, err := v.NewChallenge()
	require.ErrorIs(t, err, ErrDisabled)
}

func TestInMemorySeenStoreExpiry(t *testing.T) {
	store := NewInMemorySeenStore()
	base := time.Now()
	store.now = func() time.Time { return base }

	fresh, err := store.MarkSeen("k", time.Minute)
	require.NoError(t, err)
	require.True(t, fresh)

	fresh, err = store.MarkSeen("k", time.Minute)
	require.NoError(t, err)
	require.False(t, fresh)

	// After the entry expires the same key may be used again.
	store.now = func() time.Time { return base.Add(2 * time.Minute) }
	fresh, err = store.MarkSeen("k", time.Minute)
	require.NoError(t, err)
	require.True(t, fresh)
}

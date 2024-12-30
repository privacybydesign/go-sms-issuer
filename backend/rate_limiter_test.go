package main

import "testing"

func TestRateLimiter(t *testing.T) {
	rl := NewDefaultRateLimiter()
	ip := "127.0.0.1"
	// phone := "+31612345678"

    allow, timeRemaining := rl.Allow(ip)

    if !allow {
        t.Fatalf("first request should not be rate limited: %v", timeRemaining)
    }

    allow, timeRemaining = rl.Allow(ip)

    if !allow {
        t.Fatalf("second request should not be rate limited: %v", timeRemaining)
    }

    allow, timeRemaining = rl.Allow(ip)

    if !allow {
        t.Fatalf("third request should not be rate limited: %v", timeRemaining)
    }
}

package main

import (
	"testing"
	"time"
)

type mockClock struct {
	time time.Time
}

func (c *mockClock) GetTime() time.Time {
	return c.time
}

func (c *mockClock) IncTime(time time.Duration) {
	c.time = c.time.Add(time)
}

func between(min, max, value time.Duration) bool {
	return value >= min && value <= max
}

func TestRateLimiter(t *testing.T) {
	clock := &mockClock{time: time.Now()}

	rl := NewDefaultRateLimiter(clock)

	ip := "127.0.0.1"
	phone := "+31612345678"

	for i := 1; i <= 3; i++ {
		allow, timeRemaining := rl.Allow(ip, phone)

		if !allow {
			t.Fatalf("request %v should not be rate limited: %v", i, timeRemaining)
		}

		clock.IncTime(time.Second)
	}

	// fourth request should be rate limited
	allow, timeRemaining := rl.Allow(ip, phone)

	if allow {
		t.Fatalf("expected fourth request to be limited")
	}

	if timeRemaining.Round(time.Minute) != time.Minute {
		t.Fatalf("remaining time should be about a minute, but was %v", timeRemaining)
	}

	// half a minute later the request should still be rejected, but time remaining should have decreased
	clock.IncTime(30 * time.Second)
	allow, timeRemaining = rl.Allow(ip, phone)

	if allow {
		t.Fatalf("expected fourth request remain limited after 30 seconds")
	}

	if !between(25*time.Second, 30*time.Second, timeRemaining) {
		t.Fatalf("expected time remaining to be about 30 seconds, but was %v", timeRemaining)
	}

	// another half a minute later the timeout should be over
	clock.IncTime(30 * time.Second)
	allow, timeRemaining = rl.Allow(ip, phone)

	if !allow {
		t.Fatalf("expected timeout to be over after a minute: time remaining %v", timeRemaining)
	}

	// next request should then have a new timeout of 5 minutes
	clock.IncTime(30 * time.Second)
	allow, timeRemaining = rl.Allow(ip, phone)

	if allow {
		t.Fatalf("expected timeout after 5th request")
	}
	if timeRemaining.Round(time.Minute) != 5*time.Minute {
		t.Fatalf("expected 5 minute timeout")
	}
}

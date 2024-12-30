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

func (c *mockClock) IncTime(time.Duration) {
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
	allow, _ := rl.Allow(ip, phone)

	if allow {
		t.Fatalf("expected fourth request to be limited")
	}
}

package rate_limiter

import (
	"testing"
	"time"
)

func TestRateLimiterForMultipleClients(t *testing.T) {
	clock := &mockClock{time: time.Now()}
	rl := NewRateLimiter(NewInMemoryRateLimiterStorage(), clock, DefaultTimeoutPolicy)

	ips := []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}
	phones := []string{"+31612345678", "+31612345679", "+31612345677"}

	// first three attempts should just go by normally
	for round := 0; round < 3; round++ {
		for i := 0; i < len(ips); i++ {
			ip := ips[i]
			phone := phones[i]
			allow, remaining := rl.Allow(ip, phone)
			if !allow {
				t.Fatalf(
					"attempt %v for %v and %v was not allowed where it should: %v remaining",
					round,
					ip,
					phone,
					remaining,
				)
			}
		}
		clock.IncTime(time.Second)
	}

	// fouth attempt should give the first timeout of one minute
	for i := 0; i < len(ips); i++ {
		ip := ips[i]
		phone := phones[i]
		allow, remaining := rl.Allow(ip, phone)
		if allow {
			t.Fatalf(
				"attempt for %v and %v was allowed where it should't",
				ip,
				phone,
			)
		}
		if remaining.Round(time.Minute) != time.Minute {
			t.Fatalf(
				"timeout was expected to be 1 minute but was %v",
				remaining,
			)
		}
	}

	// after one minute you should be allowed to do another request
	clock.IncTime(time.Minute)

	for i := 0; i < len(ips); i++ {
		ip := ips[i]
		phone := phones[i]
		allow, remaining := rl.Allow(ip, phone)
		if !allow {
			t.Fatalf(
				"attempt for %v and %v was not allowed where it should, %v remaining",
				ip,
				phone,
				remaining,
			)
		}
	}
}

func TestRateLimiter(t *testing.T) {
	clock := &mockClock{time: time.Now()}

	rl := NewRateLimiter(NewInMemoryRateLimiterStorage(), clock, DefaultTimeoutPolicy)

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

package rate_limiter

import (
	"testing"
	"time"
)

func TestRateLimiterForDifferingPhonesWithSameIp(t *testing.T) {
	clock := &mockClock{time: time.Now()}
	rl := newTestRateLimiter(clock)

	ip := "127.0.0.1"
	phones := []string{
		"+31611111111",
		"+31622222222",
		"+31622222223",
		"+31622222224",
		"+31622222225",
		"+31622222226",
		"+31622222227",
		"+31622222228",
		"+31622222229",
		"+31633333333",
	}

	// 10 differing phones should be allowed to make one request each from the same ip
	for i, phone := range phones {
		allow, remaining := rl.Allow(ip, phone)

		if !allow {
			t.Fatalf("failed to allow first 10 attempts, remaining: %v (attempt %v)", remaining, i)
		}
		clock.IncTime(time.Second)
	}

	// 11th request for the same phone should not be allowed anymore
	allow, remaining := rl.Allow(ip, "+31644444444")
	if allow {
		t.Fatalf("fourth attempt for same ip should not be allowed, but is")
	}

	expectedTimeRemaining := (29 * time.Minute) + (time.Second * 50)
	if remaining.Round(time.Second) != expectedTimeRemaining {
		t.Fatalf("time remaining was expected to be %v, but is %v", expectedTimeRemaining, remaining)
	}
}

func TestRateLimiterForDifferingIpsWithSamePhone(t *testing.T) {
	clock := &mockClock{time: time.Now()}
	rl := newTestRateLimiter(clock)

	ips := []string{
		"127.0.0.1",
		"127.0.0.2",
		"127.0.0.3",
		"127.0.0.4",
		"127.0.0.5",
	}
	phone := "+31612345678"

	// three differing ips should be allowed to make one request each with the same phone
	for i, ip := range ips {
		allow, remaining := rl.Allow(ip, phone)

		if !allow {
			t.Fatalf("failed to allow first 5 attempts, remaining: %v (attempt %v)", remaining, i)
		}
		clock.IncTime(time.Second)
	}

	// fourth request for the same phone should not be allowed anymore
	allow, remaining := rl.Allow("127.0.0.6", phone)
	if allow {
		t.Fatalf("fourth attempt for same phone number should not be allowed, but is")
	}

	expectedTimeRemaining := (29 * time.Minute) + (55 * time.Second)
	if remaining.Round(time.Second) != expectedTimeRemaining {
		t.Fatalf("time remaining was expected to be %v, but is %v", expectedTimeRemaining, remaining)
	}
}

func TestRateLimiterForMultipleClients(t *testing.T) {
	clock := &mockClock{time: time.Now()}
	rl := newTestRateLimiter(clock)

	ips := []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}
	phones := []string{"+31612345678", "+31612345679", "+31612345677"}

	// first three attempts should just go by normally
	for round := 0; round < 5; round++ {
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
		clock.IncTime(time.Minute)
	}

	// 6th attempt should give a timeout of 25
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
		if remaining.Round(time.Minute) != 25*time.Minute {
			t.Fatalf(
				"timeout was expected to be 1 minute but was %v",
				remaining,
			)
		}
	}

	// after 26 minutes you should be allowed to do another request
	clock.IncTime(26 * time.Minute)

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

	rl := newTestRateLimiter(clock)

	ip := "127.0.0.1"
	phone := "+31612345678"

	for i := 1; i <= 5; i++ {
		allow, timeRemaining := rl.Allow(ip, phone)

		if !allow {
			t.Fatalf("request %v should not be rate limited: %v", i, timeRemaining)
		}

		clock.IncTime(time.Minute)
	}

	// 6th request should be rate limited
	allow, timeRemaining := rl.Allow(ip, phone)

	if allow {
		t.Fatalf("expected fourth request to be limited")
	}

	if timeRemaining.Round(time.Minute) != 25*time.Minute {
		t.Fatalf("remaining time should be about a minute, but was %v", timeRemaining)
	}

	clock.IncTime(30 * time.Minute)
	allow, timeRemaining = rl.Allow(ip, phone)

	if !allow {
		t.Fatalf("expected timeout to be over: time remaining %v", timeRemaining)
	}
}

func newTestRateLimiter(clock Clock) *TotalRateLimiter {
	ipPolicy := RateLimitingPolicy{
		Window: 30 * time.Minute,
		Limit:  10,
	}
	phonePolicy := RateLimitingPolicy{
		Window: 30 * time.Minute,
		Limit:  5,
	}

	phone := NewInMemoryRateLimiter(clock, phonePolicy)
	ip := NewInMemoryRateLimiter(clock, ipPolicy)

	return NewTotalRateLimiter(ip, phone)
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

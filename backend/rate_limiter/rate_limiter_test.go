package rate_limiter

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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
	for round := range 5 {
		for i := range ips {
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
	for i := range ips {
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

	for i := range ips {
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

func TestRedisRateLimiterAllowsUpToLimit(t *testing.T) {
	rl, _ := newTestRedisRateLimiter(t, RateLimitingPolicy{Limit: 5, Window: 30 * time.Minute})

	// a limit of 5 should allow exactly 5 requests
	for i := 1; i <= 5; i++ {
		allow, _, err := rl.Allow("phone:+31612345678")
		if err != nil {
			t.Fatalf("request %v returned unexpected error: %v", i, err)
		}
		if !allow {
			t.Fatalf("request %v should be allowed", i)
		}
	}

	// the 6th request should be blocked
	allow, timeout, err := rl.Allow("phone:+31612345678")
	if err != nil {
		t.Fatalf("blocked request returned unexpected error: %v", err)
	}
	if allow {
		t.Fatalf("6th request should be blocked")
	}
	if timeout.Round(time.Minute) != 30*time.Minute {
		t.Fatalf("retry-after should be about 30 minutes, but was %v", timeout)
	}
}

func TestRedisRateLimiterRetryAfterShrinksWithTime(t *testing.T) {
	rl, mr := newTestRedisRateLimiter(t, RateLimitingPolicy{Limit: 2, Window: 10 * time.Minute})

	for i := 1; i <= 2; i++ {
		allow, _, err := rl.Allow("phone:+31612345678")
		if err != nil || !allow {
			t.Fatalf("request %v should be allowed (err: %v)", i, err)
		}
	}

	mr.FastForward(4 * time.Minute)

	allow, timeout, err := rl.Allow("phone:+31612345678")
	if err != nil {
		t.Fatalf("blocked request returned unexpected error: %v", err)
	}
	if allow {
		t.Fatalf("3rd request should be blocked")
	}
	if timeout.Round(time.Minute) != 6*time.Minute {
		t.Fatalf("retry-after should be about 6 minutes, but was %v", timeout)
	}
}

func TestRedisRateLimiterWindowExpiryResetsCount(t *testing.T) {
	rl, mr := newTestRedisRateLimiter(t, RateLimitingPolicy{Limit: 2, Window: 10 * time.Minute})

	for i := 1; i <= 2; i++ {
		if allow, _, err := rl.Allow("phone:+31612345678"); err != nil || !allow {
			t.Fatalf("request %v should be allowed (err: %v)", i, err)
		}
	}
	if allow, _, _ := rl.Allow("phone:+31612345678"); allow {
		t.Fatalf("3rd request should be blocked")
	}

	// after the window passes the counter expires and requests are allowed again
	mr.FastForward(10 * time.Minute)

	allow, _, err := rl.Allow("phone:+31612345678")
	if err != nil {
		t.Fatalf("request after window expiry returned unexpected error: %v", err)
	}
	if !allow {
		t.Fatalf("request after window expiry should be allowed")
	}
}

func TestRedisRateLimiterKeysAreIndependent(t *testing.T) {
	rl, _ := newTestRedisRateLimiter(t, RateLimitingPolicy{Limit: 1, Window: 10 * time.Minute})

	if allow, _, err := rl.Allow("phone:+31611111111"); err != nil || !allow {
		t.Fatalf("first request for first phone should be allowed (err: %v)", err)
	}
	if allow, _, _ := rl.Allow("phone:+31611111111"); allow {
		t.Fatalf("second request for first phone should be blocked")
	}

	// a different key has its own counter
	if allow, _, err := rl.Allow("phone:+31622222222"); err != nil || !allow {
		t.Fatalf("first request for second phone should be allowed (err: %v)", err)
	}
}

func TestRedisRateLimiterNamespacesAreIndependent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	policy := RateLimitingPolicy{Limit: 1, Window: 10 * time.Minute}
	sendSms := NewRedisRateLimiter(client, "send-sms", policy)
	verifyCode := NewRedisRateLimiter(client, "verify-code", policy)

	if allow, _, err := sendSms.Allow("phone:+31612345678"); err != nil || !allow {
		t.Fatalf("first send-sms request should be allowed (err: %v)", err)
	}
	if allow, _, _ := sendSms.Allow("phone:+31612345678"); allow {
		t.Fatalf("second send-sms request should be blocked")
	}

	// the same key in another namespace has its own counter
	if allow, _, err := verifyCode.Allow("phone:+31612345678"); err != nil || !allow {
		t.Fatalf("verify-code request should not share the send-sms counter (err: %v)", err)
	}
}

func TestRedisRateLimiterRedisFailure(t *testing.T) {
	rl, mr := newTestRedisRateLimiter(t, RateLimitingPolicy{Limit: 5, Window: 10 * time.Minute})

	mr.SetError("connection lost")

	allow, _, err := rl.Allow("phone:+31612345678")
	if err == nil {
		t.Fatalf("expected an error when redis fails")
	}
	if allow {
		t.Fatalf("request should not be allowed when redis fails")
	}
}

func newTestRedisRateLimiter(t *testing.T, policy RateLimitingPolicy) (*RedisRateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisRateLimiter(client, "test", policy), mr
}

func TestRateLimiterWithIpLimitingDisabled(t *testing.T) {
	clock := &mockClock{time: time.Now()}

	ipPolicy := RateLimitingPolicy{
		Window: 30 * time.Minute,
		Limit:  10,
	}
	phonePolicy := RateLimitingPolicy{
		Window: 30 * time.Minute,
		Limit:  5,
	}

	rl := NewTotalRateLimiter(
		NewInMemoryRateLimiter(clock, ipPolicy),
		NewInMemoryRateLimiter(clock, phonePolicy),
		true,
	)

	ip := "127.0.0.1"

	// with ip limiting disabled, going way past the ip limit from a single ip
	// should still be allowed as long as each phone stays within its limit
	for i := range 30 {
		phone := fmt.Sprintf("+316%08d", i)
		allow, remaining := rl.Allow(ip, phone)
		if !allow {
			t.Fatalf("request %v should be allowed with ip limiting disabled, remaining: %v", i, remaining)
		}
		clock.IncTime(time.Second)
	}

	// the phone limit should still be enforced
	phone := "+31611111111"
	for i := 1; i <= 5; i++ {
		allow, remaining := rl.Allow(ip, phone)
		if !allow {
			t.Fatalf("request %v for phone should be allowed, remaining: %v", i, remaining)
		}
	}

	allow, _ := rl.Allow(ip, phone)
	if allow {
		t.Fatalf("6th request for the same phone should be rate limited even with ip limiting disabled")
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

	return NewTotalRateLimiter(ip, phone, false)
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

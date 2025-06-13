package rate_limiter

import (
	"time"
)

// the total rate limiter exists of one for the ip and one for the phone
type TotalRateLimiter struct {
	phone *RateLimiter
	ip    *RateLimiter
}

func NewTotalRateLimiter(ip, phone *RateLimiter) *TotalRateLimiter {
	return &TotalRateLimiter{ip: ip, phone: phone}
}

func (l *TotalRateLimiter) Allow(ip, phone string) (allow bool, timeoutRemaining time.Duration) {
	allowIp, remainingIp := l.ip.Allow(ip)
	allowPhone, remainingPhone := l.phone.Allow(phone)

	if !allowIp || !allowPhone {
		return false, max(remainingIp, remainingPhone)
	}
	return true, 0
}

type RateLimiter struct {
	storage RateLimiterStorage
	clock   Clock
	policy  TimeoutPolicy
}

// timeout policy determines what timeout you get after how many requests
type TimeoutPolicy func(numRequests int) time.Duration

func NewRateLimiter(
	storage RateLimiterStorage,
	clock Clock,
	policy TimeoutPolicy,
) *RateLimiter {
	return &RateLimiter{
		storage: storage,
		clock:   clock,
		policy:  policy,
	}
}

func (r *RateLimiter) Allow(key string) (allow bool, timeoutRemaining time.Duration) {
	r.storage.PerformTransaction(key, func(client client) client {
		now := r.clock.GetTime()

		timeSinceLastRequest := now.Sub(client.lastRequest)
		remaining := client.timeoutDuration - timeSinceLastRequest

		if remaining > 0 {
			// the timeout is not over yet, don't increment the number of attempt,
			// but also don't allow this one to pass and nofity about the remaining timeout
			allow = false
			timeoutRemaining = remaining
		} else {
			// timeout is over, pass the request, but set a new timeout
			client.numRequests += 1
			client.lastRequest = now
			client.timeoutDuration = r.policy(client.numRequests)
			allow = true
			timeoutRemaining = 0
		}
		return client
	})

	return allow, timeoutRemaining
}

func DefaultTimeoutPolicy(numRequests int) time.Duration {
	if numRequests < 3 {
		return 0
	}
	if numRequests < 4 {
		return time.Minute
	}
	if numRequests < 5 {
		return 5 * time.Minute
	}
	if numRequests < 6 {
		return time.Hour
	}
	return 24 * time.Hour
}

// to allow for testing without needing to wait for long times
type Clock interface {
	GetTime() time.Time
}

type UtcSystemClock struct{}

func NewSystemClock() *UtcSystemClock {
	return &UtcSystemClock{}
}

func (c *UtcSystemClock) GetTime() time.Time {
	return time.Now().UTC()
}

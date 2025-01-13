package rate_limiter

import (
	"time"
)

type RateLimiter struct {
	phoneStorage RateLimiterStorage
	ipStorage    RateLimiterStorage
	clock        Clock
	policy       TimeoutPolicy
}

// timeout policy determines what timeout you get after how many requests
type TimeoutPolicy func(numRequests int) time.Duration

func NewRateLimiter(
	phoneStorage RateLimiterStorage,
	ipStorage RateLimiterStorage,
	clock Clock,
	policy TimeoutPolicy,
) *RateLimiter {
	return &RateLimiter{
		phoneStorage: phoneStorage,
		ipStorage:    ipStorage,
		clock:        clock,
		policy:       policy,
	}
}

func (r *RateLimiter) Allow(ip, phone string) (allow bool, timeoutRemaining time.Duration) {
	var allowPhone bool
	var timeoutRemainingPhone time.Duration
	r.phoneStorage.PerformTransaction(phone, func(client client) client {
		now := r.clock.GetTime()

		timeSinceLastRequest := now.Sub(client.lastRequest)
		remaining := client.timeoutDuration - timeSinceLastRequest

		if remaining > 0 {
			// the timeout is not over yet, don't increment the number of attempt,
			// but also don't allow this one to pass and nofity about the remaining timeout
			allowPhone = false
			timeoutRemainingPhone = remaining
		} else {
			// timeout is over, pass the request, but set a new timeout
			client.numRequests += 1
			client.lastRequest = now
			client.timeoutDuration = r.policy(client.numRequests)
			allowPhone = true
			timeoutRemainingPhone = 0
		}
		return client
	})

	var allowIp bool
	var timeoutRemainingIp time.Duration
	r.ipStorage.PerformTransaction(ip, func(client client) client {
		now := r.clock.GetTime()

		timeSinceLastRequest := now.Sub(client.lastRequest)
		remaining := client.timeoutDuration - timeSinceLastRequest

		if remaining > 0 {
			// the timeout is not over yet, don't increment the number of attempt,
			// but also don't allow this one to pass and nofity about the remaining timeout
			allowIp = false
			timeoutRemainingIp = remaining
		} else {
			// timeout is over, pass the request, but set a new timeout
			client.numRequests += 1
			client.lastRequest = now
			client.timeoutDuration = r.policy(client.numRequests)
			allowIp = true
			timeoutRemainingIp = 0
		}
		return client
	})

	if !(allowPhone && allowIp) {
		return false, max(timeoutRemainingPhone, timeoutRemainingIp)
	}

	return true, 0
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

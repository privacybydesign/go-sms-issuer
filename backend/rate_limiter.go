package main

import (
	"fmt"
	"sync"
	"time"
)

// rate limiting policy
type RateLimiter interface {
	Allow(ip, phone string) (allow bool, timeout time.Duration)
}

type DefaultRateLimiter struct {
	mu     sync.Mutex
	limits map[string]*ClientLimiter
	clock  Clock
}

type ClientLimiter struct {
	tries            int
	timeoutDuration  time.Duration
	lastRequest      time.Time
}

func NewDefaultRateLimiter(clock Clock) *DefaultRateLimiter {
	return &DefaultRateLimiter{
		limits: make(map[string]*ClientLimiter),
		clock:  clock,
	}
}

func getTimeout(tries int) time.Duration {
    if tries < 3 {
        return 0
    }
    if tries < 4 {
        return time.Minute
    }
    if tries < 5 {
        return 5 * time.Minute
    }
    if tries < 6 {
        return time.Hour
    }
    return 24 * time.Hour
}

func (r *DefaultRateLimiter) Allow(ip, phone string) (allow bool, timeout time.Duration) {
	client := r.getClientLimiter(ip, phone)
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	// if the number of requests is less than 3, let it pass
	if client.tries < 3 {
		client.tries += 1
		client.lastRequest = now
		return true, 0
	}

	// tries is over 3
	timeSinceLastRequest := now.Sub(client.lastRequest)
	remaining := client.timeoutDuration - timeSinceLastRequest

	if remaining > 0 {
		// the timeout is not over yet, don't increment the number of attempt,
        // but also don't allow this one to pass and nofity about the remaining timeout
		return false, remaining
	} else {
		// timeout is over, pass the request, but set a new timeout
        client.tries += 1
        client.lastRequest = now
        client.timeoutDuration = getTimeout(client.tries)
        return true, 0
	}
}

func (r *DefaultRateLimiter) getClientLimiter(ip, phone string) *ClientLimiter {
	key := fmt.Sprintf("%v&%v", ip, phone)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.limits[key]; !exists {
		r.limits[key] = &ClientLimiter{
			tries:            0,
			timeoutDuration:  1 * time.Minute,
		}
	}
	return r.limits[key]
}

// to allow for testing without needing to wait for long times
type Clock interface {
	GetTime() time.Time
}

type SystemClock struct{}

func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

func (c *SystemClock) GetTime() time.Time {
	return time.Now()
}

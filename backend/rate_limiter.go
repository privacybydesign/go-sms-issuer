package main

import (
	"fmt"
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	limits map[string]*Client
	clock  Clock
	policy func(attempts int) time.Duration
}

type Client struct {
	numRequests     int
	timeoutDuration time.Duration
	lastRequest     time.Time
}

// timeout policy determines what timeout you get after how many requests
type TimeoutPolicy func(numRequests int) time.Duration

func NewRateLimiter(clock Clock, policy TimeoutPolicy) *RateLimiter {
	return &RateLimiter{
		limits: make(map[string]*Client),
		clock:  clock,
		policy: policy,
	}
}

// returns whether the request made by the ip & phone combo is allowed at this moment
// if it's now allowed it will also return the timeout duration remaining
func (r *RateLimiter) Allow(ip, phone string) (allow bool, timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	client := r.getClient(ip, phone)
	now := r.clock.GetTime()

	// tries is 3 or higher
	timeSinceLastRequest := now.Sub(client.lastRequest)
	remaining := client.timeoutDuration - timeSinceLastRequest

	if remaining > 0 {
		// the timeout is not over yet, don't increment the number of attempt,
		// but also don't allow this one to pass and nofity about the remaining timeout
		return false, remaining
	} else {
		// timeout is over, pass the request, but set a new timeout
		client.numRequests += 1
		client.lastRequest = now
		client.timeoutDuration = r.policy(client.numRequests)
		return true, 0
	}
}

func (r *RateLimiter) getClient(ip, phone string) *Client {
	key := fmt.Sprintf("%v&%v", ip, phone)

	if _, exists := r.limits[key]; !exists {
		r.limits[key] = &Client{
			numRequests: 0,
		}
	}
	return r.limits[key]
}

func DefaultTimeoutPolicy(tries int) time.Duration {
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

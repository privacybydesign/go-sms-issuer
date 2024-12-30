package main

import (
	"fmt"
	"sync"
	"time"
)

type InMemoryRateLimiterStorage struct {
	mu     sync.Mutex
	limits map[string]*Client
}

func NewInMemoryRateLimiterStorage() *InMemoryRateLimiterStorage {
	return &InMemoryRateLimiterStorage{
		limits: make(map[string]*Client),
	}
}

func (s *InMemoryRateLimiterStorage) PerformTransaction(ip, phone string, tx ClientTransaction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%v&%v", ip, phone)

	if _, exists := s.limits[key]; !exists {
		s.limits[key] = &Client{
			numRequests: 0,
		}
	}
    client := s.limits[key]
    *client = tx(*client)
}

// a function that alters the client
type ClientTransaction func(client Client) Client

type RateLimiterStorage interface {
	PerformTransaction(ip, phone string, tx ClientTransaction)
}

type RateLimiter struct {
	storage RateLimiterStorage
	clock   Clock
	policy  func(attempts int) time.Duration
}

type Client struct {
	numRequests     int
	timeoutDuration time.Duration
	lastRequest     time.Time
}

// timeout policy determines what timeout you get after how many requests
type TimeoutPolicy func(numRequests int) time.Duration

func NewRateLimiter(storage RateLimiterStorage, clock Clock, policy TimeoutPolicy) *RateLimiter {
	return &RateLimiter{
		storage: storage,
		clock:   clock,
		policy:  policy,
	}
}

// returns whether the request made by the ip & phone combo is allowed at this moment
// if it's now allowed it will also return the timeout duration remaining
func (r *RateLimiter) Allow(ip, phone string) (allow bool, timeoutRemaining time.Duration) {
	r.storage.PerformTransaction(ip, phone, func(client Client) Client {
		now := r.clock.GetTime()

		// tries is 3 or higher
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

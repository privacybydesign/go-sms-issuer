package main

import (
	"sync"
	"time"
)

// rate limiting policy
type RateLimiter interface {
	GetTimeoutSecsFor(ip, phone string) float64
}

type Client struct {
	requests    int
	timeout     time.Duration
	lastRequest time.Time
}

func NewDefaultRateLimiter() *DefaultRateLimiter {
	return &DefaultRateLimiter{
		clients: make(map[string]*Client),
		mutex:   sync.Mutex{},
		rate:    3,
		window:  24 * time.Hour,
	}
}

type DefaultRateLimiter struct {
	clients map[string]*Client
	mutex   sync.Mutex
	rate    int
	window  time.Duration
}

func (rl *DefaultRateLimiter) GetTimeoutSecsFor(ip, phone string) float64 {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	client, exists := rl.clients[ip]

	if !exists {
        client = &Client{
			requests:    0,
			timeout:     0,
			lastRequest: now,
		}
		rl.clients[ip] = client
	}

	expiryTime := client.lastRequest.Add(client.timeout)
	timeRemaining := now.Sub(expiryTime)

	// the clients timeout has not expired yet
	if timeRemaining > 0 {
		return timeRemaining.Seconds()
	}

    // reset the requests if the window has passed
    if now.Sub(client.lastRequest) > rl.window {
        client.requests = 0
    }

	client.requests += 1
	client.lastRequest = now

	if client.requests >= rl.rate {
		client.timeout = 100 * time.Second
	}

	return 0.0
}

type NeverRateLimiter struct {}

func (rl *NeverRateLimiter) GetTimeoutSecsFor(ip, phone string) float64 {
    return 0.0
}

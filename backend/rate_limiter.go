package main

import (
	"sync"
	"time"
)

// rate limiting policy
type RateLimiter interface {
    Allow(ip string) (allow bool, timeout time.Duration)
}

type DefaultRateLimiter struct {
    mu sync.Mutex 
    limits map[string]*ClientLimiter
}

type ClientLimiter struct {
    tries int
    timeoutDuration time.Duration
    lastRequest time.Time
    nextTimeoutLevel time.Duration
}

func NewDefaultRateLimiter() *DefaultRateLimiter {
    return &DefaultRateLimiter {
        limits: make(map[string]*ClientLimiter),
    }
}

func (r *DefaultRateLimiter) getClientLimiter(ip string) *ClientLimiter {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.limits[ip]; !exists {
        r.limits[ip] = &ClientLimiter {
            tries: 3,
            timeoutDuration: 1 * time.Minute,
            nextTimeoutLevel: 5 * time.Minute,
        }
    }
    return r.limits[ip]
}

func (r *DefaultRateLimiter) Allow(ip string) (allow bool, timeout time.Duration) {
    clientLimiter := r.getClientLimiter(ip)
    now := time.Now()

    r.mu.Lock()
    defer r.mu.Unlock()

    if now.After(clientLimiter.lastRequest.Add(clientLimiter.timeoutDuration)) {
        clientLimiter.tries = 3
        clientLimiter.timeoutDuration = 1 * time.Minute
        return true, 0
    }

    if clientLimiter.tries > 0 {
        clientLimiter.tries -= 1
        clientLimiter.lastRequest = now
        return true, 0
    }

    remaining := clientLimiter.lastRequest.Add(clientLimiter.timeoutDuration).Sub(now)

    if remaining <= 0 {
        clientLimiter.tries = 0
        clientLimiter.lastRequest = now
        clientLimiter.timeoutDuration = clientLimiter.nextTimeoutLevel
        return true, 0
    }

    return false, remaining
}


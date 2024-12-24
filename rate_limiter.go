package main

// rate limiting policy
type RateLimiter interface {
    GetTimeoutSecsFor(ip, phone string) float32 
}

type DefaultRateLimiter struct {

}

func (rl *DefaultRateLimiter) GetTimeoutSecsFor(ip, phone string) float32 {
    return 0.0
}


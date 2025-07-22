package rate_limiter

import (
	"context"
	"fmt"
	"go-sms-issuer/logging"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter interface {
	Allow(key string) (allow bool, timeout time.Duration, err error)
}

type RateLimitingPolicy struct {
	Limit  int           `json:"limit"`
	Window time.Duration `json:"window"`
}

type RedisRateLimiter struct {
	namespace string
	client    *redis.Client
	ctx       context.Context
	policy    RateLimitingPolicy
}

func NewRedisRateLimiter(redis *redis.Client, namespace string, policy RateLimitingPolicy) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: redis,
		ctx:    context.Background(),
		policy: policy,
	}
}

func (r *RedisRateLimiter) Allow(key string) (bool, time.Duration, error) {
	key = fmt.Sprintf("%s:%s", r.namespace, key)
	count, err := r.client.Incr(r.ctx, key).Result()
	if err != nil {
		logging.Error.Printf("Redis Incr failed: %v\n %v\n", err, key)
		return false, 0, err
	}
	if count == 1 {
		// First request: set expiry
		err = r.client.Expire(r.ctx, key, r.policy.Window).Err()
		if err != nil {
			logging.Error.Printf("Redis Expire failed: %v\n", err)
			return false, 0, err
		}
	}
	if count >= int64(r.policy.Limit) {
		timeRemaining, err := r.client.TTL(r.ctx, key).Result()
		if err != nil {
			return false, 0, err
		}
		return false, timeRemaining, nil
	}
	return true, 0, nil
}

type ratelimiterEntry struct {
	count  int
	expiry time.Time
}

type InMemoryRateLimiter struct {
	memory map[string]*ratelimiterEntry
	mutex  sync.Mutex
	policy RateLimitingPolicy
	clock  Clock
}

func NewInMemoryRateLimiter(clock Clock, policy RateLimitingPolicy) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		memory: map[string]*ratelimiterEntry{},
		mutex:  sync.Mutex{},
		policy: policy,
		clock:  clock,
	}
}

func (r *InMemoryRateLimiter) Allow(key string) (allow bool, timeout time.Duration, err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	entry, exists := r.memory[key]

	if !exists {
		r.memory[key] = &ratelimiterEntry{
			count:  0,
			expiry: r.clock.GetTime().Add(r.policy.Window),
		}
		entry = r.memory[key]
	}

	entry.count += 1

	if entry.count > r.policy.Limit {
		timeUntilExpiry := entry.expiry.Sub(r.clock.GetTime())

		if timeUntilExpiry < 0 {
			entry.expiry = r.clock.GetTime().Add(r.policy.Window)
			entry.count = 0
			return true, 0, nil
		}
		return false, timeUntilExpiry, nil
	}
	return true, 0, nil

}

// the total rate limiter exists of one for the ip and one for the phone
type TotalRateLimiter struct {
	phone RateLimiter
	ip    RateLimiter
}

func NewTotalRateLimiter(ip, phone RateLimiter) *TotalRateLimiter {
	return &TotalRateLimiter{ip: ip, phone: phone}
}

func (l *TotalRateLimiter) Allow(ip, phone string) (allow bool, timeoutRemaining time.Duration) {
	ipKey := fmt.Sprintf("ip:%s", ip)
	phoneKey := fmt.Sprintf("phone:%s", phone)

	allowPhone, timeRemainingPhone, err := l.phone.Allow(phoneKey)
	if err != nil {
		return false, 30 * time.Minute
	}

	allowIp, timeRemainingIp, err := l.ip.Allow(ipKey)
	if err != nil {
		return false, 30 * time.Minute
	}

	if !allowIp || !allowPhone {
		return false, max(timeRemainingIp, timeRemainingPhone)
	}
	return true, 0
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

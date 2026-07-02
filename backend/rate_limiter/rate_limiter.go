package rate_limiter

import (
	"context"
	"fmt"
	"go-sms-issuer/logging"
	"log/slog"
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
		client:    redis,
		ctx:       context.Background(),
		policy:    policy,
		namespace: namespace,
	}
}

func (r *RedisRateLimiter) Allow(key string) (bool, time.Duration, error) {
	key = fmt.Sprintf("%s:%s", r.namespace, key)
	maskedKey := logging.MaskKey(key)
	count, err := r.client.Incr(r.ctx, key).Result()
	if err != nil {
		slog.Error("rate limiter: redis INCR failed", "key", maskedKey, "error", err)
		return false, 0, err
	}
	if count == 1 {
		// First request: set expiry
		err = r.client.Expire(r.ctx, key, r.policy.Window).Err()
		if err != nil {
			slog.Error("rate limiter: redis EXPIRE failed", "key", maskedKey, "error", err)
			return false, 0, err
		}
	}
	// block once the limit is exceeded, so a limit of 5 allows exactly
	// 5 requests per window (same semantics as InMemoryRateLimiter)
	if count > int64(r.policy.Limit) {
		timeRemaining, err := r.client.TTL(r.ctx, key).Result()
		if err != nil {
			slog.Error("rate limiter: redis TTL failed", "key", maskedKey, "error", err)
			return false, 0, err
		}
		slog.Debug("rate limiter: limit reached",
			"key", maskedKey,
			"count", count,
			"limit", r.policy.Limit,
			"window", r.policy.Window,
			"retry_after", timeRemaining)
		return false, timeRemaining, nil
	}
	slog.Debug("rate limiter: request allowed",
		"key", maskedKey,
		"count", count,
		"limit", r.policy.Limit,
		"window", r.policy.Window)
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
	maskedKey := logging.MaskKey(key)
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
		slog.Debug("rate limiter: limit reached",
			"key", maskedKey,
			"count", entry.count,
			"limit", r.policy.Limit,
			"window", r.policy.Window,
			"retry_after", timeUntilExpiry)
		return false, timeUntilExpiry, nil
	}
	slog.Debug("rate limiter: request allowed",
		"key", maskedKey,
		"count", entry.count,
		"limit", r.policy.Limit,
		"window", r.policy.Window)
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
	maskedPhone := logging.MaskPhone(phone)

	// Check the IP limit first and short-circuit on denial. This keeps a
	// blocked IP from also consuming the phone quota, which an attacker
	// could otherwise abuse to lock a victim's phone number out by burning
	// its per-phone budget from an already rate-limited IP.
	allowIp, timeRemainingIp, err := l.ip.Allow(ipKey)
	if err != nil {
		slog.Error("rate limiter: ip check failed, denying request",
			"ip", ip,
			"phone", maskedPhone,
			"error", err)
		return false, 30 * time.Minute
	}

	if !allowIp {
		slog.Warn("rate limit exceeded",
			"limited_by", "ip",
			"ip", ip,
			"phone", maskedPhone,
			"retry_after", timeRemainingIp)
		return false, timeRemainingIp
	}

	allowPhone, timeRemainingPhone, err := l.phone.Allow(phoneKey)
	if err != nil {
		slog.Error("rate limiter: phone check failed, denying request",
			"ip", ip,
			"phone", maskedPhone,
			"error", err)
		return false, 30 * time.Minute
	}

	if !allowPhone {
		slog.Warn("rate limit exceeded",
			"limited_by", "phone",
			"ip", ip,
			"phone", maskedPhone,
			"retry_after", timeRemainingPhone)
		return false, timeRemainingPhone
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

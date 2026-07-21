package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisSeenStore is an altcha.SeenStore backed by Redis, so single-use tracking
// works across multiple issuer instances. It uses SETNX so a key can only be
// claimed once within its ttl.
type RedisSeenStore struct {
	client    *redis.Client
	namespace string
}

func NewRedisSeenStore(client *redis.Client, namespace string) *RedisSeenStore {
	return &RedisSeenStore{client: client, namespace: fmt.Sprintf("%s:altcha-seen", namespace)}
}

func (s *RedisSeenStore) MarkSeen(key string, ttl time.Duration) (bool, error) {
	// A non-positive ttl means the challenge has already expired; treat it as
	// used so it can never be spent.
	if ttl <= 0 {
		return false, nil
	}
	ctx := context.Background()
	ok, err := s.client.SetNX(ctx, fmt.Sprintf("%s:%s", s.namespace, key), 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to mark challenge as seen: %w", err)
	}
	return ok, nil
}

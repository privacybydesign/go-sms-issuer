package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type InMemoryTokenStorage struct {
	TokenMap    map[string]string
	AttemptsMap map[string]int
	mutex       sync.Mutex
}

func NewInMemoryTokenStorage() *InMemoryTokenStorage {
	return &InMemoryTokenStorage{
		TokenMap:    make(map[string]string),
		AttemptsMap: make(map[string]int),
	}
}

type RedisTokenStorage struct {
	client    *redis.Client
	namespace string
}

func NewRedisTokenStorage(client *redis.Client, namespace string) *RedisTokenStorage {
	return &RedisTokenStorage{client: client, namespace: namespace}
}

// Should be safe to use in concurreny
type TokenStorage interface {
	// Store given token for the given phone number,
	// returns an error when it somehow fails to store the value.
	// Should not return an error when the value already exists,
	// it should just update in that case.
	StoreToken(phone, token string) error

	// Should retrieve the token for the given phone number
	// and return an error in any case where it fails to do so.
	RetrieveToken(phone string) (string, error)

	// Should remove the token and return an error if it fails to do so.
	// The value not being there should also be considered an error.
	RemoveToken(phone string) error

	// Increments the failed-verify counter for the given phone and returns the
	// new count. Used by handleVerify to invalidate tokens after too many wrong
	// submissions; see MaxFailedAttempts.
	IncrementFailedAttempts(phone string) (int, error)
}

// MaxFailedAttempts is the number of wrong-token submissions allowed for a
// single stored token before it is invalidated and a fresh /send is required.
const MaxFailedAttempts = 5

// ------------------------------------------------------------------------------

func createKey(namespace, phone string) string {
	return fmt.Sprintf("%s:token:%s", namespace, phone)
}

const Timeout time.Duration = 10 * time.Minute

func createAttemptsKey(namespace, phone string) string {
	return fmt.Sprintf("%s:token-attempts:%s", namespace, phone)
}

func (s *RedisTokenStorage) StoreToken(phone, token string) error {
	ctx := context.Background()
	// Reset any stale attempt counter from a previous token for this phone.
	if err := s.client.Del(ctx, createAttemptsKey(s.namespace, phone)).Err(); err != nil {
		return err
	}
	return s.client.Set(ctx, createKey(s.namespace, phone), token, Timeout).Err()
}

func (s *RedisTokenStorage) RetrieveToken(phone string) (string, error) {
	ctx := context.Background()
	return s.client.Get(ctx, createKey(s.namespace, phone)).Result()
}

func (s *RedisTokenStorage) RemoveToken(phone string) error {
	ctx := context.Background()
	if err := s.client.Del(ctx, createAttemptsKey(s.namespace, phone)).Err(); err != nil {
		return err
	}
	return s.client.Del(ctx, createKey(s.namespace, phone)).Err()
}

func (s *RedisTokenStorage) IncrementFailedAttempts(phone string) (int, error) {
	ctx := context.Background()
	key := createAttemptsKey(s.namespace, phone)
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		// First failure: align expiry with the token itself so the counter
		// can never outlive its associated token.
		if err := s.client.Expire(ctx, key, Timeout).Err(); err != nil {
			return int(count), err
		}
	}
	return int(count), nil
}

// ------------------------------------------------------------------------------

func (s *InMemoryTokenStorage) StoreToken(phone, token string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.TokenMap[phone] = token
	// Reset any stale attempt counter from a previous token for this phone.
	delete(s.AttemptsMap, phone)
	return nil
}

func (s *InMemoryTokenStorage) RetrieveToken(phone string) (string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if token, ok := s.TokenMap[phone]; ok {
		return token, nil
	} else {
		return "", fmt.Errorf("failed to find token for %s", phone)
	}
}

func (s *InMemoryTokenStorage) RemoveToken(phone string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.AttemptsMap, phone)
	if _, ok := s.TokenMap[phone]; ok {
		delete(s.TokenMap, phone)
		return nil
	} else {
		return fmt.Errorf("failed to remove token for %s, because it wasn't there", phone)
	}
}

func (s *InMemoryTokenStorage) IncrementFailedAttempts(phone string) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.AttemptsMap[phone]++
	return s.AttemptsMap[phone], nil
}

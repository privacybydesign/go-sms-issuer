package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type InMemoryTokenStorage struct {
	TokenMap map[string]string
	mutex    sync.Mutex
}

func NewInMemoryTokenStorage() *InMemoryTokenStorage {
	return &InMemoryTokenStorage{
		TokenMap: make(map[string]string),
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
}

// ------------------------------------------------------------------------------

func createKey(namespace, phone string) string {
	return fmt.Sprintf("%s:token:%s", namespace, phone)
}

const Timeout time.Duration = 24 * time.Hour

func (s *RedisTokenStorage) StoreToken(phone, token string) error {
	ctx := context.Background()
	return s.client.Set(ctx, createKey(s.namespace, phone), token, Timeout).Err()
}

func (s *RedisTokenStorage) RetrieveToken(phone string) (string, error) {
	ctx := context.Background()
	return s.client.Get(ctx, createKey(s.namespace, phone)).Result()
}

func (s *RedisTokenStorage) RemoveToken(phone string) error {
	ctx := context.Background()
	return s.client.Del(ctx, createKey(s.namespace, phone)).Err()
}

// ------------------------------------------------------------------------------

func (s *InMemoryTokenStorage) StoreToken(phone, token string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.TokenMap[phone] = token
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

	if _, ok := s.TokenMap[phone]; ok {
		delete(s.TokenMap, phone)
		return nil
	} else {
		return fmt.Errorf("failed to remove token for %s, because it wasn't there", phone)
	}
}

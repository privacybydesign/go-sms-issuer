package main

import (
	"fmt"
	"sync"
)

type InMemoryTokenRepository struct {
	TokenMap map[string]string
	mutex    sync.Mutex
}

func NewInMemoryTokenRepo() *InMemoryTokenRepository {
	return &InMemoryTokenRepository{
		TokenMap: make(map[string]string),
	}
}

// should be safe to use in concurreny
type TokenRepository interface {
	StoreToken(phone string, token string) error
	RetrieveToken(phone string) (string, error)
	RemoveToken(phone string) error
}

func (s *InMemoryTokenRepository) StoreToken(phone string, token string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.TokenMap[phone] = token
	return nil
}

func (s *InMemoryTokenRepository) RetrieveToken(phone string) (string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if token, ok := s.TokenMap[phone]; ok {
		return token, nil
	} else {
		return "", fmt.Errorf("failed to find token for %s", phone)
	}
}

func (s *InMemoryTokenRepository) RemoveToken(phone string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.TokenMap[phone]; ok {
		delete(s.TokenMap, phone)
		return nil
	} else {
		return fmt.Errorf("failed to remove token for %s, because it wasn't there", phone)
	}
}

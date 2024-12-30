package rate_limiter

import (
	"fmt"
	"sync"
	"time"
)

type client struct {
	numRequests     int
	timeoutDuration time.Duration
	lastRequest     time.Time
}

type InMemoryRateLimiterStorage struct {
	mu     sync.Mutex
	limits map[string]*client
}

func NewInMemoryRateLimiterStorage() *InMemoryRateLimiterStorage {
	return &InMemoryRateLimiterStorage{
		limits: make(map[string]*client),
	}
}

func (s *InMemoryRateLimiterStorage) PerformTransaction(ip, phone string, tx clientTransaction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%v&%v", ip, phone)

	if _, exists := s.limits[key]; !exists {
		s.limits[key] = &client{
			numRequests: 0,
		}
	}
	client := s.limits[key]
	*client = tx(*client)
}

// a function that alters the client
type clientTransaction func(client client) client

type RateLimiterStorage interface {
	PerformTransaction(ip, phone string, tx clientTransaction)
}

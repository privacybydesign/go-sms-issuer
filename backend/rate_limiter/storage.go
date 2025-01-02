package rate_limiter

import (
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

func (s *InMemoryRateLimiterStorage) PerformTransaction(clientId string, tx clientTransaction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.limits[clientId]; !exists {
		s.limits[clientId] = &client{
			numRequests: 0,
		}
	}
	client := s.limits[clientId]
	*client = tx(*client)
}

// a function that alters the client
type clientTransaction func(client client) client

type RateLimiterStorage interface {
	PerformTransaction(clientId string, tx clientTransaction)
}

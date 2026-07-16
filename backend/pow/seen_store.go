package pow

import (
	"sync"
	"time"
)

// InMemorySeenStore is a SeenStore backed by a map, for single-instance
// deployments and tests. Entries expire on their own so the map cannot grow
// without bound; expired entries are also pruned lazily on each call.
type InMemorySeenStore struct {
	mu   sync.Mutex
	seen map[string]time.Time
	now  func() time.Time
}

func NewInMemorySeenStore() *InMemorySeenStore {
	return &InMemorySeenStore{
		seen: make(map[string]time.Time),
		now:  time.Now,
	}
}

func (s *InMemorySeenStore) MarkSeen(key string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.pruneExpired(now)

	if expiry, ok := s.seen[key]; ok && now.Before(expiry) {
		return false, nil
	}
	s.seen[key] = now.Add(ttl)
	return true, nil
}

// pruneExpired drops entries whose expiry has passed. The caller holds the lock.
func (s *InMemorySeenStore) pruneExpired(now time.Time) {
	for k, expiry := range s.seen {
		if !now.Before(expiry) {
			delete(s.seen, k)
		}
	}
}

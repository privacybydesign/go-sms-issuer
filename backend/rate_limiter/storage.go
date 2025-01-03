package rate_limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	log "go-sms-issuer/logging"
)

type client struct {
	numRequests     int
	timeoutDuration time.Duration
	lastRequest     time.Time
}

type RedisConfig struct {
	SentinelHost     string `json:"sentinel_host"`
	SentinelPort     int    `json:"sentinel_port"`
	Password         string `json:"password"`
	MasterName       string `json:"master_name"`
	SentinelUsername string `json:"sentinel_username"`
}

type RedisRateLimiterStorage struct {
	client *redis.Client
}

func NewRedisRateLimiterStorage(config RedisConfig) (*RedisRateLimiterStorage, error) {
	ctx := context.Background()

	addr := fmt.Sprintf("%v:%v", config.SentinelHost, config.SentinelPort)
	sentinelOptions := &redis.FailoverOptions{
		MasterName:       config.MasterName,
		SentinelAddrs:    []string{addr},
		Password:         config.Password,
		SentinelUsername: config.SentinelUsername,
		DB:               0,
		DialTimeout:      5 * time.Second,
	}

	client := redis.NewFailoverClient(sentinelOptions)
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis through Sentinel: %w", err)
	}

	return &RedisRateLimiterStorage{client: client}, nil
}

const (
	numRequestsRedisKey = "num-requests"
	timeoutSecsRedisKey = "timeout-secs"
	lastRequestRedisKey = "last-request"
)

func clientToRedis(tx *redis.Tx, ctx context.Context, key string, client client) error {
	err := tx.HSet(ctx, key, numRequestsRedisKey, client.numRequests).Err()

	if err != nil {
		return fmt.Errorf("failed to set num-requests to redis: %v", err)
	}

	timeoutDuration := client.timeoutDuration.String()
	err = tx.HSet(ctx, key, timeoutSecsRedisKey, timeoutDuration).Err()

	if err != nil {
		return fmt.Errorf("failed to set timeout to redis: %v", err)
	}

	lastRequest := client.lastRequest.String()
	err = tx.HSet(ctx, key, lastRequestRedisKey, lastRequest).Err()

	if err != nil {
		return fmt.Errorf("failed to set last-request to redis: %v", err)
	}

	err = tx.Expire(ctx, key, 48*time.Hour).Err()

	if err != nil {
		return fmt.Errorf("failed to set expiration time in redis: %v", err)
	}

	return nil
}

func clientFromRedis(tx *redis.Tx, ctx context.Context, key string) (client, error) {
	numRequests, err := tx.HGet(ctx, key, numRequestsRedisKey).Int()

	if err != nil {
		return client{}, fmt.Errorf("failed to get num-requests from redis: %v", err)
	}

	timeoutDurationStr, err := tx.HGet(ctx, key, timeoutSecsRedisKey).Result()

	if err != nil {
		return client{}, fmt.Errorf("failed to get timeout from redis: %v", err)
	}

	timeoutDuration, err := time.ParseDuration(timeoutDurationStr)

	if err != nil {
		return client{}, fmt.Errorf("failed to parse timeout duration: %v", err)
	}

	lastRequestStr, err := tx.HGet(ctx, key, lastRequestRedisKey).Result()

	if err != nil {
		return client{}, fmt.Errorf("failed to get last-request from redis: %v", err)
	}

	layout := "2006-01-02 15:04:05.999999999 -0700 MST"
	lastRequest, err := time.Parse(layout, lastRequestStr)

	if err != nil {
		return client{}, fmt.Errorf("failed to parse last-request as time: %v", err)
	}

	return client{
		numRequests:     numRequests,
		timeoutDuration: timeoutDuration,
		lastRequest:     lastRequest,
	}, nil

}

func (s *RedisRateLimiterStorage) PerformTransaction(clientId string, transaction clientTransaction) {
	ctx := context.Background()
	key := fmt.Sprintf("sms-issuer:rate-limiter:%v", clientId)
	err := s.client.Watch(ctx, func(rtx *redis.Tx) error {
		client, err := clientFromRedis(rtx, ctx, key)

		if err != nil {
			return fmt.Errorf("failed to get client from redis: %v", err)
		}

		client = transaction(client)

		err = clientToRedis(rtx, ctx, key, client)
		if err != nil {
			return fmt.Errorf("failed to store client to redis: %v", err)
		}

		return nil
	}, key)

	if err != nil {
		// do some logging
		log.Error.Printf("failed to perform redis transaction: %v", err)
	}
}

// ------------------------------------------

type InMemoryRateLimiterStorage struct {
	mu     sync.Mutex
	limits map[string]*client
}

func NewInMemoryRateLimiterStorage() *InMemoryRateLimiterStorage {
	return &InMemoryRateLimiterStorage{
		limits: make(map[string]*client),
	}
}

// this is not part of the RateLimiterStorage api, so it needs to be called from somewhere else
func (s *InMemoryRateLimiterStorage) RemoveOutdated() {
	toRemove := make([]string, 7)
	for key, value := range s.limits {
		if time.Now().Sub(value.lastRequest) > 48*time.Hour {
			toRemove = append(toRemove, key)
		}
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

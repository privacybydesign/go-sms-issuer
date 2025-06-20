package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	redis "go-sms-issuer/redis"
	"os"
	"time"
)

type Config struct {
	ServerConfig ServerConfig `json:"server_config"`

	JwtPrivateKeyPath string `json:"jwt_private_key_path"`
	IrmaServerUrl     string `json:"irma_server_url"`
	IssuerId          string `json:"issuer_id"`
	FullCredential    string `json:"full_credential"`
	Attribute         string `json:"attribute"`

	SmsTemplates        map[string]string         `json:"sms_templates"`
	SmsBackend          string                    `json:"sms_backend"`
	CmSmsSenderConfig   CmSmsSenderConfig         `json:"cm_sms_sender_config,omitempty"`
	StorageType         string                    `json:"storage_type"`
	RedisConfig         redis.RedisConfig         `json:"redis_config,omitempty"`
	RedisSentinelConfig redis.RedisSentinelConfig `json:"redis_sentinel_config,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "Path for the config.json to use")
	flag.Parse()

	if *configPath == "" {
		log.Error.Fatal("please provide a config path using the --config flag")
	}

	log.Info.Printf("using config: %v", *configPath)

	config, err := readConfigFile(*configPath)
	if err != nil {
		log.Error.Fatalf("failed to read config file: %v", err)
	}

	log.Info.Printf("hosting on: %v:%v", config.ServerConfig.Host, config.ServerConfig.Port)

	jwtCreator, err := NewIrmaJwtCreator(
		config.JwtPrivateKeyPath,
		config.IssuerId,
		config.FullCredential,
		config.Attribute,
	)
	if err != nil {
		log.Error.Fatalf("failed to instantiate jwt creator: %v", err)
	}

	smsSender, err := createSmsBackend(&config)
	if err != nil {
		log.Error.Fatalf("failed to instantiate sms backend: %v", err)
	}

	rateLimiter, err := createRateLimiter(&config)
	if err != nil {
		log.Error.Fatalf("failed to instantiate rate limiter: %v", err)
	}

	tokenStorage, err := createTokenStorage(&config)
	if err != nil {
		log.Error.Fatalf("failed to instantiate token storage: %v", err)
	}

	serverState := ServerState{
		irmaServerURL:  config.IrmaServerUrl,
		tokenStorage:   tokenStorage,
		smsSender:      smsSender,
		jwtCreator:     jwtCreator,
		tokenGenerator: NewRandomTokenGenerator(),
		smsTemplates:   config.SmsTemplates,
		rateLimiter:    rateLimiter,
	}

	server, err := NewServer(&serverState, config.ServerConfig)
	if err != nil {
		log.Error.Fatalf("failed to create server: %v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Error.Fatalf("failed to listen and serve: %v", err)
	}
}

func createTokenStorage(config *Config) (TokenStorage, error) {
	if config.StorageType == "redis" {
		client, err := redis.NewRedisClient(&config.RedisConfig)
		if err != nil {
			return nil, err
		}
		return NewRedisTokenStorage(client), nil
	}
	if config.StorageType == "redis_sentinel" {
		client, err := redis.NewRedisSentinelClient(&config.RedisSentinelConfig)
		if err != nil {
			return nil, err
		}
		return NewRedisTokenStorage(client), nil
	}
	if config.StorageType == "memory" {
		return NewInMemoryTokenStorage(), nil
	}
	return nil, fmt.Errorf("%v is not a valid storage type", config.StorageType)
}

func createRateLimiter(config *Config) (*rate.TotalRateLimiter, error) {
	ipRateLimitingPolicy := rate.RateLimitingPolicy{
		Limit:  10,
		Window: 30 * time.Minute,
	}
	phoneRateLimitingPolicy := rate.RateLimitingPolicy{
		Limit:  5,
		Window: 30 * time.Minute,
	}

	if config.StorageType == "redis" {
		client, err := redis.NewRedisClient(&config.RedisConfig)
		if err != nil {
			return nil, err
		}
		return rate.NewTotalRateLimiter(
			rate.NewRedisRateLimiter(client, ipRateLimitingPolicy),
			rate.NewRedisRateLimiter(client, phoneRateLimitingPolicy),
		), nil
	}
	if config.StorageType == "redis_sentinel" {
		client, err := redis.NewRedisSentinelClient(&config.RedisSentinelConfig)
		if err != nil {
			return nil, err
		}
		return rate.NewTotalRateLimiter(
			rate.NewRedisRateLimiter(client, ipRateLimitingPolicy),
			rate.NewRedisRateLimiter(client, phoneRateLimitingPolicy),
		), nil
	}
	if config.StorageType == "memory" {
		return rate.NewTotalRateLimiter(
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), ipRateLimitingPolicy),
			rate.NewInMemoryRateLimiter(rate.NewSystemClock(), phoneRateLimitingPolicy),
		), nil
	}
	return nil, errors.New("no valid storage type was set")
}

func createSmsBackend(config *Config) (SmsSender, error) {
	if config.SmsBackend == "dummy" {
		return &DummySmsSender{}, nil
	}
	if config.SmsBackend == "cm" {
		return NewCmSmsSender(config.CmSmsSenderConfig)
	}
	return nil, fmt.Errorf("invalid sms backend: %v", config.SmsBackend)
}

func readConfigFile(path string) (Config, error) {
	configBytes, err := os.ReadFile(path)

	if err != nil {
		return Config{}, err
	}

	var config Config
	err = json.Unmarshal(configBytes, &config)

	if err != nil {
		return Config{}, err
	}

	return config, nil
}

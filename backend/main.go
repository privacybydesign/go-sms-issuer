package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	"os"
)

type Config struct {
	ServerConfig ServerConfig `json:"server_config"`

	JwtPrivateKeyPath string `json:"jwt_private_key_path"`
	IssuerId          string `json:"issuer_id"`
	FullCredential    string `json:"full_credential"`
	Attribute         string `json:"attribute"`

	SmsTemplates        map[string]string        `json:"sms_templates"`
	SmsBackend          string                   `json:"sms_backend"`
	CmSmsSenderConfig   CmSmsSenderConfig        `json:"cm_sms_sender_config,omitempty"`
	StorageType         string                   `json:"storage_type"`
	RedisConfig         rate.RedisConfig         `json:"redis_config,omitempty"`
	RedisSentinelConfig rate.RedisSentinelConfig `json:"redis_sentinel_config,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "Path for the config.json to use")
	flag.Parse()

	if *configPath == "" {
		log.Error.Fatal("please provide a config path using the --config flag")
	}

	log.Info.Printf("using config: %v", *configPath)

	config, err := readConfigFile(*configPath)

	log.Info.Printf("%v\n", config)

	if err != nil {
		log.Error.Fatalf("failed to read config file: %v", err)
	}

	log.Info.Printf("hosting on: %v:%v", config.ServerConfig.Host, config.ServerConfig.Port)

	jwtCreator, err := NewDefaultJwtCreator(
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
		log.Error.Fatalf("failed to create redis storage: %v", err)
	}

	serverState := ServerState{
		tokenRepo:      NewInMemoryTokenRepo(),
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

func createRateLimiter(config *Config) (*rate.RateLimiter, error) {
	if config.StorageType == "redis" {
		client, err := rate.NewRedisClient(&config.RedisConfig)
		if err != nil {
			return nil, err
		}
		ipStorage := rate.NewRedisRateLimiterStorage(client)
		phoneStorage := rate.NewRedisRateLimiterStorage(client)
		return rate.NewRateLimiter(phoneStorage, ipStorage, rate.NewSystemClock(), rate.DefaultTimeoutPolicy), nil
	}
	if config.StorageType == "redis_sentinel" {
		client, err := rate.NewRedisSentinelClient(&config.RedisSentinelConfig)
		if err != nil {
			return nil, err
		}
		ipStorage := rate.NewRedisRateLimiterStorage(client)
		phoneStorage := rate.NewRedisRateLimiterStorage(client)
		return rate.NewRateLimiter(phoneStorage, ipStorage, rate.NewSystemClock(), rate.DefaultTimeoutPolicy), nil
	}
	if config.StorageType == "memory" {
		ipStorage := rate.NewInMemoryRateLimiterStorage()
		phoneStorage := rate.NewInMemoryRateLimiterStorage()
		return rate.NewRateLimiter(phoneStorage, ipStorage, rate.NewSystemClock(), rate.DefaultTimeoutPolicy), nil
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

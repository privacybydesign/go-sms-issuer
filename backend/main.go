package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	log "go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	"os"
	"strconv"
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

	storage, err := createRateLimiterStorage(&config)
	if err != nil {
		log.Error.Fatalf("failed to create redis storage: %v", err)
	}

	serverState := ServerState{
		tokenRepo:      NewInMemoryTokenRepo(),
		smsSender:      smsSender,
		jwtCreator:     jwtCreator,
		tokenGenerator: &RandomTokenGenerator{},
		smsTemplates:   config.SmsTemplates,
		rateLimiter: rate.NewRateLimiter(
			storage,
			rate.NewSystemClock(),
			rate.DefaultTimeoutPolicy,
		),
	}

	server, err := NewServer(serverState, config.ServerConfig)

	if err != nil {
		log.Error.Fatalf("failed to create server: %v", err)
	}

	err = server.ListenAndServe()

	if err != nil {
		log.Error.Fatalf("failed to listen and serve: %v", err)
	}
}

func redisConfigFromEnv() (*rate.RedisSentinelConfig, error) {
	port, err := strconv.Atoi(os.Getenv("REDIS_SENTINEL_PORT"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse port for redis sentinel: %v", err)
	}

	host := os.Getenv("REDIS_SENTINEL_HOST")
	password := os.Getenv("REDIS_PASSWORD")
	sentinelUsername := os.Getenv("REDIS_SENTINEL_USERNAME")
	masterName := os.Getenv("REDIS_MASTER_NAME")

	if password == "" {
		return nil, errors.New("redis password is empty")
	}

	if masterName == "" {
		return nil, errors.New("redis masterName is empty")
	}

	return &rate.RedisSentinelConfig{
		SentinelHost:     host,
		SentinelPort:     port,
		SentinelUsername: sentinelUsername,
		MasterName:       masterName,
		Password:         password,
	}, nil
}

func createRateLimiterStorage(config *Config) (rate.RateLimiterStorage, error) {
	if config.StorageType == "redis" {
		return rate.NewRedisRateLimiterStorage(&config.RedisConfig)
	}
	if config.StorageType == "redis_sentinel" {
		return rate.NewRedisSentinelRateLimiterStorage(&config.RedisSentinelConfig)
	}
	if config.StorageType == "memory" {
		return rate.NewInMemoryRateLimiterStorage(), nil
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

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go-sms-issuer/logging"
	rate "go-sms-issuer/rate_limiter"
	redis "go-sms-issuer/redis"
	turnstile "go-sms-issuer/turnstile"
	"log/slog"
	"os"
	"time"
)

type Config struct {
	ServerConfig ServerConfig `json:"server_config"`

	// LogLevel is deserialized by slog.Level itself: it accepts "debug",
	// "info", "warn" and "error" (case-insensitive). An unknown value makes
	// readConfigFile fail, so typos abort startup instead of silently
	// running at info. When the key is absent the zero value is info.
	LogLevel          slog.Level `json:"log_level"`
	JwtPrivateKeyPath string     `json:"jwt_private_key_path"`
	IrmaServerUrl     string     `json:"irma_server_url"`
	IssuerId          string     `json:"issuer_id"`
	FullCredential    string     `json:"full_credential"`
	Attribute         string     `json:"attribute"`

	SmsTemplates           map[string]string                `json:"sms_templates"`
	SmsBackend             string                           `json:"sms_backend"`
	CmSmsSenderConfig      CmSmsSenderConfig                `json:"cm_sms_sender_config"`
	StorageType            string                           `json:"storage_type"`
	RedisConfig            redis.RedisConfig                `json:"redis_config"`
	RedisSentinelConfig    redis.RedisSentinelConfig        `json:"redis_sentinel_config"`
	TurnStileBackend       string                           `json:"turnstile_backend,omitempty"`
	TurnStileConfiguration turnstile.TurnStileConfiguration `json:"turnstile_configuration"`
}

func main() {
	configPath := flag.String("config", "", "Path for the config.json to use")
	flag.Parse()

	if *configPath == "" {
		slog.Error("please provide a config path using the --config flag")
		os.Exit(1)
	}

	config, err := readConfigFile(*configPath)
	if err != nil {
		slog.Error("failed to read config file", "error", err)
		os.Exit(1)
	}

	logging.InitLogger(config.LogLevel)

	slog.Info("using config", "path", *configPath)
	slog.Info("hosting on", "host", config.ServerConfig.Host, "port", config.ServerConfig.Port)

	jwtCreator, err := NewIrmaJwtCreator(
		config.JwtPrivateKeyPath,
		config.IssuerId,
		config.FullCredential,
		config.Attribute,
	)
	if err != nil {
		slog.Error("failed to instantiate jwt creator", "error", err)
		os.Exit(1)
	}

	smsSender, err := createSmsBackend(&config)
	if err != nil {
		slog.Error("failed to instantiate sms backend", "error", err)
		os.Exit(1)
	}

	turnstileVerifier, err := createTurnstileValidator(&config)
	if err != nil {
		slog.Error("failed to instantiate turnstile verifier", "error", err)
		os.Exit(1)
	}

	sendSmsRateLimiter, err := createSendSmsRateLimiter(&config)
	if err != nil {
		slog.Error("failed to instantiate rate limiter for sending sms", "error", err)
		os.Exit(1)
	}

	verifyCodeRateLimiter, err := createVerifyCodeRateLimiter(&config)
	if err != nil {
		slog.Error("failed to instantiate rate limiter for verifying codes", "error", err)
		os.Exit(1)
	}

	tokenStorage, err := createTokenStorage(&config)
	if err != nil {
		slog.Error("failed to instantiate token storage", "error", err)
		os.Exit(1)
	}

	serverState := ServerState{
		irmaServerURL:         config.IrmaServerUrl,
		tokenStorage:          tokenStorage,
		smsSender:             smsSender,
		jwtCreator:            jwtCreator,
		tokenGenerator:        NewRandomTokenGenerator(),
		smsTemplates:          config.SmsTemplates,
		sendSmsRateLimiter:    sendSmsRateLimiter,
		verifyCodeRateLimiter: verifyCodeRateLimiter,
		turnstileVerifier:     turnstileVerifier,
	}

	server, err := NewServer(&serverState, config.ServerConfig)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	err = server.ListenAndServe()
	if err != nil {
		slog.Error("failed to listen and serve", "error", err)
		os.Exit(1)
	}
}

func createTokenStorage(config *Config) (TokenStorage, error) {
	if config.StorageType == "redis" {
		slog.Info("Using redis token storage")
		client, err := redis.NewRedisClient(&config.RedisConfig)
		if err != nil {
			return nil, err
		}
		return NewRedisTokenStorage(client, config.RedisConfig.Namespace), nil
	}
	if config.StorageType == "redis_sentinel" {
		slog.Info("Using redis sentinal storage")
		client, err := redis.NewRedisSentinelClient(&config.RedisSentinelConfig)
		if err != nil {
			return nil, err
		}
		return NewRedisTokenStorage(client, config.RedisSentinelConfig.Namespace), nil
	}
	if config.StorageType == "memory" {
		slog.Info("Using in memory storage")
		return NewInMemoryTokenStorage(), nil
	}
	return nil, fmt.Errorf("%v is not a valid storage type", config.StorageType)
}

func createSendSmsRateLimiter(config *Config) (*rate.TotalRateLimiter, error) {
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
		redisNamespace := fmt.Sprintf("%s:send-sms", config.RedisConfig.Namespace)
		return rate.NewTotalRateLimiter(
			rate.NewRedisRateLimiter(client, redisNamespace, ipRateLimitingPolicy),
			rate.NewRedisRateLimiter(client, redisNamespace, phoneRateLimitingPolicy),
		), nil
	}
	if config.StorageType == "redis_sentinel" {
		client, err := redis.NewRedisSentinelClient(&config.RedisSentinelConfig)
		if err != nil {
			return nil, err
		}
		redisNamespace := fmt.Sprintf("%s:send-sms", config.RedisSentinelConfig.Namespace)
		return rate.NewTotalRateLimiter(
			rate.NewRedisRateLimiter(client, redisNamespace, ipRateLimitingPolicy),
			rate.NewRedisRateLimiter(client, redisNamespace, phoneRateLimitingPolicy),
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

func createVerifyCodeRateLimiter(config *Config) (*rate.TotalRateLimiter, error) {
	ipRateLimitingPolicy := rate.RateLimitingPolicy{
		Limit:  25,
		Window: 30 * time.Minute,
	}
	phoneRateLimitingPolicy := rate.RateLimitingPolicy{
		Limit:  25,
		Window: 30 * time.Minute,
	}

	if config.StorageType == "redis" {
		client, err := redis.NewRedisClient(&config.RedisConfig)
		if err != nil {
			return nil, err
		}
		redisNamespace := fmt.Sprintf("%s:verify-code", config.RedisConfig.Namespace)
		return rate.NewTotalRateLimiter(
			rate.NewRedisRateLimiter(client, redisNamespace, ipRateLimitingPolicy),
			rate.NewRedisRateLimiter(client, redisNamespace, phoneRateLimitingPolicy),
		), nil
	}
	if config.StorageType == "redis_sentinel" {
		client, err := redis.NewRedisSentinelClient(&config.RedisSentinelConfig)
		if err != nil {
			return nil, err
		}
		redisNamespace := fmt.Sprintf("%s:verify-code", config.RedisSentinelConfig.Namespace)
		return rate.NewTotalRateLimiter(
			rate.NewRedisRateLimiter(client, redisNamespace, ipRateLimitingPolicy),
			rate.NewRedisRateLimiter(client, redisNamespace, phoneRateLimitingPolicy),
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

func createTurnstileValidator(config *Config) (turnstile.TurnStileVerifier, error) {
	if config.TurnStileBackend == "dummy" {
		slog.Info("using dummy turnstile validator")
		return &turnstile.MockTurnStileValidator{Success: true}, nil
	}
	if config.TurnStileBackend == "turnstile" {
		slog.Info("using cloudflare turnstile validator")
		if config.TurnStileConfiguration.SecretKey == "" || config.TurnStileConfiguration.SiteKey == "" {
			return nil, errors.New("turnstile secret key and site key must be set for turnstile backend")
		}
		if config.TurnStileConfiguration.ApiUrl == "" {
			config.TurnStileConfiguration.ApiUrl = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
		}
		return turnstile.NewTurnStileValidator(config.TurnStileConfiguration), nil
	}
	return nil, fmt.Errorf("invalid turnstile backend")
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

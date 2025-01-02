package main

import (
	"encoding/json"
	"flag"
	"fmt"
	rate "go-sms-issuer/rate_limiter"
	"os"
)

type Config struct {
	ServerConfig ServerConfig `json:"server_config"`

	JwtPrivateKeyPath string `json:"jwt_private_key_path"`
	IssuerId          string `json:"issuer_id"`
	FullCredential    string `json:"full_credential"`
	Attribute         string `json:"attribute"`

	SmsTemplates      map[string]string `json:"sms_templates"`
	SmsBackend        string            `json:"sms_backend"`
	CmSmsSenderConfig CmSmsSenderConfig `json:"cm_sms_sender_config,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "Path for the config.json to use")
	flag.Parse()

	if *configPath == "" {
		ErrorLogger.Fatal("please provide a config path using the --config flag")
	}

	InfoLogger.Printf("using config: %v", *configPath)

	config, err := readConfigFile(*configPath)

	if err != nil {
		ErrorLogger.Fatalf("failed to read config file: %v", err)
	}

	InfoLogger.Printf("hosting on: %v:%v", config.ServerConfig.Host, config.ServerConfig.Port)

	jwtCreator, err := NewDefaultJwtCreator(
		config.JwtPrivateKeyPath,
		config.IssuerId,
		config.FullCredential,
		config.Attribute,
	)

	if err != nil {
		ErrorLogger.Fatalf("failed to instantiate jwt creator: %v", err)
	}

	smsSender, err := createSmsBackend(&config)

	if err != nil {
		ErrorLogger.Fatalf("failed to instantiate sms backend: %v", err)
	}

	serverState := ServerState{
		tokenRepo:      NewInMemoryTokenRepo(),
		smsSender:      smsSender,
		jwtCreator:     jwtCreator,
		tokenGenerator: &RandomTokenGenerator{},
		smsTemplates:   config.SmsTemplates,
		rateLimiter: rate.NewRateLimiter(
			rate.NewInMemoryRateLimiterStorage(),
			rate.NewSystemClock(),
			rate.DefaultTimeoutPolicy,
		),
	}

	server, err := NewServer(serverState, config.ServerConfig)

	if err != nil {
		ErrorLogger.Fatalf("failed to create server: %v", err)
	}

	err = server.ListenAndServe()

	if err != nil {
		ErrorLogger.Fatalf("failed to listen and serve: %v", err)
	}
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

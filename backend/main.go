package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
    rate "go-sms-issuer/rate_limiter"
)

type Config struct {
	ServerConfig ServerConfig `json:"server_config"`

	JwtPrivateKeyPath string `json:"jwt_private_key_path"`
	IssuerId          string `json:"issuer_id"`
	FullCredential    string `json:"full_credential"`
	Attribute         string `json:"attribute"`

	SmsTemplates      map[string]string `json:"sms_templates"`
	CmSmsSenderConfig CmSmsSenderConfig `json:"cm_sms_sender_config"`
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

	serverState := ServerState{
		tokenRepo:      NewInMemoryTokenRepo(),
		smsSender:      &CmSmsSender{config.CmSmsSenderConfig},
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

func readConfigFile(path string) (Config, error) {
	configFile, err := os.Open(path)

	if err != nil {
		return Config{}, err
	}

	configContent, err := io.ReadAll(configFile)

	if err != nil {
		return Config{}, err
	}

	var config Config
	err = json.Unmarshal([]byte(configContent), &config)

	if err != nil {
		return Config{}, err
	}

	return config, nil
}

package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "irma-sms-issuer",
	Short: "The sms issuer for Yivi",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cobra run in root cmd")
	},
}

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
	config := Config{
		ServerConfig: ServerConfig{
			Host:           "127.0.0.1",
			Port:           8080,
			UseTls:         false,
			TlsPrivKeyPath: "",
			TlsCertPath:    "",
		},
		JwtPrivateKeyPath: ".secrets/private.pem",
		IssuerId:          "sms_issuer",
		FullCredential:    "irma-demo.sidn-pbdf.mobilenumber",
		Attribute:         "mobilenumber",
		SmsTemplates: map[string]string{
			"en": "Yivi verification code: %s\nOr directly via a link:\nhttps://sms-issuer.staging.yivi.app/en/#!verify:%s",
			"nl": "Yivi verificatiecode: %s\nOf direct via een link:\nhttps://sms-issuer.staging.yivi.app/nl/#!verify:%s",
		},
		CmSmsSenderConfig: CmSmsSenderConfig{
			From:         "",
			ApiEndpoint:  "",
			ProductToken: "",
			Reference:    "",
		},
	}

	jwtCreator, err := NewDefaultJwtCreator(
		config.JwtPrivateKeyPath,
		config.IssuerId,
		config.FullCredential,
		config.Attribute,
	)

	if err != nil {
		ErrorLogger.Fatalf("failed to instantiate jwt creator: %v", err)
	}

	deps := ServerState{
		tokenRepo:      NewInMemoryTokenRepo(),
		smsSender:      &CmSmsSender{config.CmSmsSenderConfig},
		jwtCreator:     jwtCreator,
		tokenGenerator: &DefaultTokenGenerator{},
		smsTemplates:   config.SmsTemplates,
	}

	server, err := NewServer(deps, config.ServerConfig)

	if err != nil {
		ErrorLogger.Fatalf("failed to instantiate jwt creator: %v", err)
	}

	err = server.ListenAndServe()

	if err != nil {
		ErrorLogger.Fatalf("failed to instantiate jwt creator: %v", err)
	}
}

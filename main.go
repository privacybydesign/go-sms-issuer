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

func main() {
	config := ServerConfig{
		Host: "127.0.0.1",
		Port: 8080,
	}

	server := Server{
		config:    config,
		tokenRepo: NewInMemoryTokenRepo(),
		smsSender: &CmSmsSender{
			From:         "",
			ApiEndpoint:  "",
			ProductToken: "",
			Reference:    "",
			SmsTemplates: map[string]string{},
		},
	}

	StartServer(server)
}

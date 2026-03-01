package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	AnthropicAPIKey string
	SessionSecret   string
	IsDev           bool
	NeedsSetup      bool // true if no API key found at startup
}

func Load() Config {
	if os.Getenv("ENV") != "production" {
		_ = godotenv.Load()
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" && os.Getenv("ENV") == "production" {
		log.Fatal("SESSION_SECRET is required in production")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	return Config{
		Port:            port,
		AnthropicAPIKey: key,
		SessionSecret:   secret,
		IsDev:           os.Getenv("ENV") != "production",
		NeedsSetup:      key == "",
	}
}

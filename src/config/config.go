package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	AnthropicAPIKey     string
	SessionSecret       string
	SteuerparameterPath string
	IsDev               bool
}

func Load() Config {
	if os.Getenv("ENV") != "production" {
		_ = godotenv.Load()
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" && os.Getenv("ENV") == "production" {
		log.Fatal("SESSION_SECRET is required in production")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	path := os.Getenv("STEUERPARAMETER_PATH")
	if path == "" {
		path = "./docs/steuerparameter.json"
	}
	return Config{
		Port:                port,
		AnthropicAPIKey:     key,
		SessionSecret:       secret,
		SteuerparameterPath: path,
		IsDev:               os.Getenv("ENV") != "production",
	}
}

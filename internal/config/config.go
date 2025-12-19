package config

import (
	"errors"
	"os"
)

// Config holds the bot configuration
type Config struct {
	DiscordToken string
	DatabaseURL  string
	OwnerID      string
	LogLevel     string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return nil, errors.New("DISCORD_TOKEN environment variable is required")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL environment variable is required")
	}

	return &Config{
		DiscordToken: token,
		DatabaseURL:  dbURL,
		OwnerID:      os.Getenv("OWNER_ID"),
		LogLevel:     os.Getenv("LOG_LEVEL"),
	}, nil
}

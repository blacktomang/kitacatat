// Package config loads and validates the bot's runtime configuration from the
// environment (optionally seeded by a .env file via godotenv).
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the typed, validated configuration for the bot.
type Config struct {
	TelegramBotToken string
	GeminiAPIKey     string
	DatabaseURL      string
	TessdataPrefix   string
}

// Load reads configuration from the environment. It first attempts to load a
// .env file (ignored if absent, so production env-vars work unchanged), then
// validates that every required value is present and well-formed.
func Load() (*Config, error) {
	// Best-effort: load a local .env, then fall back to the repo-root .env
	// (so `turbo dev` from apps/bot still finds the shared file). Missing
	// files are fine — real environment variables take over in production,
	// and godotenv never overrides variables already set in the environment.
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")

	cfg := &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		TessdataPrefix:   os.Getenv("TESSDATA_PREFIX"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	var missing []string
	if c.TelegramBotToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if c.GeminiAPIKey == "" {
		missing = append(missing, "GEMINI_API_KEY")
	}
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

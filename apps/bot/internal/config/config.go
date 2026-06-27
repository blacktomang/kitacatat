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

	// AI provider selection (provider-agnostic).
	AIProvider string // "gemini" (default) or "openai" (OpenAI-compatible)
	AIModel    string
	AIAPIKey   string
	AIBaseURL  string // required for AIProvider="openai"

	DatabaseURL    string
	TessdataPrefix string

	// DashboardURL is the base URL of the dashboard, used to build the /login
	// link the bot DMs to users.
	DashboardURL string
}

const defaultDashboardURL = "http://localhost:5173"

const (
	defaultProvider = "gemini"
	// DefaultGeminiModel has free-tier quota; gemini-2.0-flash does not on newer
	// keys (and is being retired).
	DefaultGeminiModel = "gemini-2.5-flash"
)

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

	provider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if provider == "" {
		provider = defaultProvider
	}

	cfg := &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		AIProvider:       provider,
		// AI_MODEL / AI_API_KEY are the canonical names; GEMINI_MODEL /
		// GEMINI_API_KEY are accepted as back-compat fallbacks.
		AIModel:        firstNonEmpty(os.Getenv("AI_MODEL"), os.Getenv("GEMINI_MODEL")),
		AIAPIKey:       firstNonEmpty(os.Getenv("AI_API_KEY"), os.Getenv("GEMINI_API_KEY")),
		AIBaseURL:      os.Getenv("AI_BASE_URL"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		TessdataPrefix: os.Getenv("TESSDATA_PREFIX"),
		// TrimSpace guards against a stray trailing space in the env var, which
		// would otherwise leak into the login link (".../pages.dev /?token=...").
		DashboardURL:   strings.TrimSpace(firstNonEmpty(os.Getenv("DASHBOARD_URL"), defaultDashboardURL)),
	}
	if cfg.AIProvider == "gemini" && cfg.AIModel == "" {
		cfg.AIModel = DefaultGeminiModel
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
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.AIAPIKey == "" {
		missing = append(missing, "AI_API_KEY (or GEMINI_API_KEY)")
	}
	if c.AIModel == "" {
		missing = append(missing, "AI_MODEL")
	}
	if c.AIProvider == "openai" && c.AIBaseURL == "" {
		missing = append(missing, "AI_BASE_URL (required when AI_PROVIDER=openai)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	switch c.AIProvider {
	case "gemini", "openai":
	default:
		return fmt.Errorf("invalid AI_PROVIDER %q (want \"gemini\" or \"openai\")", c.AIProvider)
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

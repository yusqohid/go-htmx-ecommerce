package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration variables for the application.
type Config struct {
	AppEnv            string
	AppPort           string
	AppBaseURL        string
	DatabaseURL       string
	SessionSecret     string
	StoragePath       string
	PaymentProvider   string
	LynkAPIKey       string
	LynkWebhookSecret string
	LynkBaseURL       string
}

// Load loads configuration from .env and system environment variables.
func Load() (*Config, error) {
	// Try loading .env file, ignore error if file not found (e.g. in container/prod environments)
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:            getEnv("APP_ENV", "development"),
		AppPort:           getEnv("APP_PORT", "8080"),
		AppBaseURL:        getEnv("APP_BASE_URL", "http://localhost:8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/sellora?sslmode=disable"),
		SessionSecret:     getEnv("SESSION_SECRET", "default-dev-secret-key-must-change-in-production"),
		StoragePath:       getEnv("STORAGE_PATH", "./storage/products"),
		PaymentProvider:   strings.ToLower(getEnv("PAYMENT_PROVIDER", "mock")),
		LynkAPIKey:       getEnv("LYNK_API_KEY", ""),
		LynkWebhookSecret: getEnv("LYNK_WEBHOOK_SECRET", ""),
		LynkBaseURL:       getEnv("LYNK_BASE_URL", "https://api.lynk.id"),
	}

	if cfg.IsProduction() && (cfg.SessionSecret == "" || cfg.SessionSecret == "default-dev-secret-key-must-change-in-production") {
		return nil, fmt.Errorf("SESSION_SECRET must be set to a secure string in production")
	}

	return cfg, nil
}

// IsProduction returns true if APP_ENV is set to "production".
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.AppEnv) == "production"
}

// getEnv gets an environment variable with a fallback default value.
func getEnv(key, defaultValue string) string {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return defaultValue
	}
	return strings.TrimSpace(val)
}

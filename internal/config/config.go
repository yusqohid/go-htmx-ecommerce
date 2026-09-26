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
	LynkAPIKey           string
	LynkWebhookSecret     string
	LynkBaseURL           string
	MidtransServerKey     string
	MidtransClientKey     string
	MidtransIsProduction bool
	MidtransSnapURL       string
	SMTPHost              string
	SMTPPort              string
	SMTPUsername          string
	SMTPPassword          string
	SMTPFromEmail         string
	SMTPFromName          string
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
		LynkAPIKey:           getEnv("LYNK_API_KEY", ""),
		LynkWebhookSecret:     getEnv("LYNK_WEBHOOK_SECRET", ""),
		LynkBaseURL:           getEnv("LYNK_BASE_URL", "https://api.lynk.id"),
		MidtransServerKey:     getEnv("MIDTRANS_SERVER_KEY", ""),
		MidtransClientKey:     getEnv("MIDTRANS_CLIENT_KEY", ""),
		MidtransIsProduction: strings.ToLower(getEnv("MIDTRANS_IS_PRODUCTION", "false")) == "true",
		MidtransSnapURL:       getEnv("MIDTRANS_SNAP_URL", ""),
		SMTPHost:              getEnv("SMTP_HOST", ""),
		SMTPPort:              getEnv("SMTP_PORT", "587"),
		SMTPUsername:          getEnv("SMTP_USERNAME", ""),
		SMTPPassword:          getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail:         getEnv("SMTP_FROM_EMAIL", "no-reply@sellora.local"),
		SMTPFromName:          getEnv("SMTP_FROM_NAME", "Sellora"),
	}
	if cfg.IsProduction() && (cfg.SessionSecret == "" || cfg.SessionSecret == "default-dev-secret-key-must-change-in-production") {
		return nil, fmt.Errorf("SESSION_SECRET must be set to a secure string in production")
	}

	if cfg.IsProduction() && cfg.PaymentProvider == "mock" {
		return nil, fmt.Errorf("PAYMENT_PROVIDER 'mock' is strictly prohibited in production mode")
	}

	if cfg.PaymentProvider == "midtrans" && cfg.MidtransServerKey == "" {
		return nil, fmt.Errorf("MIDTRANS_SERVER_KEY must be provided when payment provider is set to 'midtrans'")
	}

	if cfg.MidtransSnapURL == "" {
		if cfg.MidtransIsProduction {
			cfg.MidtransSnapURL = "https://app.midtrans.com/snap/v1/transactions"
		} else {
			cfg.MidtransSnapURL = "https://app.sandbox.midtrans.com/snap/v1/transactions"
		}
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

package config_test

import (
	"os"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Clean up any test env vars
	_ = os.Unsetenv("APP_ENV")
	_ = os.Unsetenv("APP_PORT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error loading default config, got %v", err)
	}

	if cfg.AppPort == "" {
		t.Errorf("expected default AppPort, got empty string")
	}

	if cfg.IsProduction() {
		t.Errorf("expected IsProduction to be false by default")
	}
}

func TestConfigProductionValidation(t *testing.T) {
	_ = os.Setenv("APP_ENV", "production")
	_ = os.Setenv("SESSION_SECRET", "")
	defer func() {
		_ = os.Unsetenv("APP_ENV")
		_ = os.Unsetenv("SESSION_SECRET")
	}()

	_, err := config.Load()
	if err == nil {
		t.Errorf("expected error when SESSION_SECRET is empty in production mode")
	}
}

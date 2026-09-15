package payment

import (
	"fmt"
	"strings"

	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// Provider is an alias to domain.PaymentProvider for convenience within the payment package.
type Provider = domain.PaymentProvider

// NewProvider creates a payment provider implementation based on configuration.
func NewProvider(cfg *config.Config) (domain.PaymentProvider, error) {
	switch strings.ToLower(cfg.PaymentProvider) {
	case "lynk":
		if cfg.LynkAPIKey == "" {
			return nil, fmt.Errorf("LYNK_API_KEY must be provided when payment provider is set to 'lynk'")
		}
		return NewLynkProvider(cfg.LynkAPIKey, cfg.LynkWebhookSecret, cfg.LynkBaseURL, cfg.AppBaseURL), nil
	case "mock", "":
		return NewMockProvider(cfg.AppBaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported payment provider: %s", cfg.PaymentProvider)
	}
}

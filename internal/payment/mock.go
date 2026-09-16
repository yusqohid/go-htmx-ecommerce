package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// MockProvider provides local development payment simulation without external gateway dependencies.
type MockProvider struct {
	appBaseURL string
}

// NewMockProvider constructs a MockProvider.
func NewMockProvider(appBaseURL string) *MockProvider {
	return &MockProvider{
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
	}
}

// Name returns the provider identifier.
func (p *MockProvider) Name() string {
	return "mock"
}

// CreateCheckout generates a local URL for simulating the checkout process.
func (p *MockProvider) CreateCheckout(ctx context.Context, order *domain.Order) (string, error) {
	if order == nil {
		return "", fmt.Errorf("order cannot be nil")
	}
	return fmt.Sprintf("%s/mock-checkout?ref=%s", p.appBaseURL, order.Reference), nil
}

// VerifyWebhook parses a mock webhook event payload.
func (p *MockProvider) VerifyWebhook(r *http.Request) (*domain.WebhookEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read mock webhook body: %w", err)
	}

	var payload struct {
		EventID        string `json:"event_id"`
		OrderReference string `json:"order_reference"`
		Status         string `json:"status"` // "paid" or "failed"
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid mock webhook payload: %w", err)
	}

	var status domain.OrderStatus
	switch payload.Status {
	case "paid":
		status = domain.StatusPaid
	case "failed":
		status = domain.StatusFailed
	default:
		return nil, fmt.Errorf("unsupported mock webhook status: %q", payload.Status)
	}

	return &domain.WebhookEvent{
		Provider:         p.Name(),
		EventID:          payload.EventID,
		EventType:        "mock.payment." + payload.Status,
		OrderReference:   payload.OrderReference,
		Status:           status,
		PaymentReference: "MOCK-PAY-" + payload.EventID,
		RawPayload:       body,
	}, nil
}

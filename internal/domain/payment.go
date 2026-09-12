package domain

import (
	"context"
	"net/http"
	"time"
)

// PaymentEvent represents an incoming webhook event from an external payment provider.
type PaymentEvent struct {
	ID             int64     `json:"id"`
	Provider       string    `json:"provider"`
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	OrderReference string    `json:"order_reference"`
	Payload        []byte    `json:"payload"`
	ProcessedAt    time.Time `json:"processed_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// PaymentEventRepository defines persistence operations for recording payment events idempotently.
type PaymentEventRepository interface {
	Record(ctx context.Context, event *PaymentEvent) error
	Exists(ctx context.Context, provider, eventID string) (bool, error)
}

// WebhookEvent represents a normalized webhook payload parsed from any payment provider.
type WebhookEvent struct {
	Provider         string
	EventID          string
	EventType        string
	OrderReference   string
	Status           OrderStatus
	PaymentReference string
	RawPayload       []byte
}

// PaymentProvider defines the abstract contract that every payment gateway must satisfy.
type PaymentProvider interface {
	Name() string
	CreateCheckout(ctx context.Context, order *Order) (checkoutURL string, err error)
	VerifyWebhook(r *http.Request) (*WebhookEvent, error)
}

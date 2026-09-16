package payment

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// OrderService defines the methods required from the order domain to fulfill webhooks.
type OrderService interface {
	GetOrderByReference(ctx context.Context, ref string) (*domain.Order, error)
	UpdateOrderStatus(ctx context.Context, id int64, status domain.OrderStatus, paymentRef string) error
}

// Service manages payment processing business logic, webhook verification, and idempotency.
type Service struct {
	provider     domain.PaymentProvider
	eventRepo    domain.PaymentEventRepository
	orderService OrderService
}

// NewService constructs a new payment Service.
func NewService(provider domain.PaymentProvider, eventRepo domain.PaymentEventRepository, orderService OrderService) *Service {
	return &Service{
		provider:     provider,
		eventRepo:    eventRepo,
		orderService: orderService,
	}
}

// ProcessWebhook verifies an incoming payment event, enforces idempotency, and transitions order status.
func (s *Service) ProcessWebhook(ctx context.Context, providerName string, r *http.Request) error {
	if s.provider.Name() != providerName {
		return fmt.Errorf("provider %q does not match active provider %q", providerName, s.provider.Name())
	}

	event, err := s.provider.VerifyWebhook(r)
	if err != nil {
		return fmt.Errorf("failed to verify webhook: %w", err)
	}

	// Idempotency check: if event has already been processed, acknowledge without duplicate fulfillment
	alreadyProcessed, err := s.eventRepo.Exists(ctx, event.Provider, event.EventID)
	if err != nil {
		return fmt.Errorf("failed to check payment event idempotency: %w", err)
	}
	if alreadyProcessed {
		return nil
	}

	// Resolve corresponding order
	ord, err := s.orderService.GetOrderByReference(ctx, event.OrderReference)
	if err != nil {
		return fmt.Errorf("order not found for reference %s: %w", event.OrderReference, err)
	}

	// Transition order status
	if err := s.orderService.UpdateOrderStatus(ctx, ord.ID, event.Status, event.PaymentReference); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Record payment event for future idempotency checks
	pe := &domain.PaymentEvent{
		Provider:       event.Provider,
		EventID:        event.EventID,
		EventType:      event.EventType,
		OrderReference: event.OrderReference,
		Payload:        event.RawPayload,
		ProcessedAt:    time.Now().UTC(),
	}
	if err := s.eventRepo.Record(ctx, pe); err != nil {
		return fmt.Errorf("failed to record payment event: %w", err)
	}

	return nil
}

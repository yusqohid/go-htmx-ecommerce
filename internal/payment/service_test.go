package payment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/payment"
)

type mockOrderService struct {
	mu           sync.Mutex
	orders       map[string]*domain.Order
	updateCounts map[int64]int
}

func newMockOrderService() *mockOrderService {
	return &mockOrderService{
		orders:       make(map[string]*domain.Order),
		updateCounts: make(map[int64]int),
	}
}

func (m *mockOrderService) GetOrderByReference(ctx context.Context, ref string) (*domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ord, ok := m.orders[ref]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return ord, nil
}

func (m *mockOrderService) UpdateOrderStatus(ctx context.Context, id int64, status domain.OrderStatus, paymentRef string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ord := range m.orders {
		if ord.ID == id {
			ord.Status = status
			ord.PaymentReference = paymentRef
			m.updateCounts[id]++
			return nil
		}
	}
	return domain.ErrNotFound
}

type mockEventRepo struct {
	mu     sync.Mutex
	events map[string]*domain.PaymentEvent
}

func newMockEventRepo() *mockEventRepo {
	return &mockEventRepo{
		events: make(map[string]*domain.PaymentEvent),
	}
}

func (m *mockEventRepo) Record(ctx context.Context, event *domain.PaymentEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := event.Provider + ":" + event.EventID
	m.events[key] = event
	return nil
}

func (m *mockEventRepo) Exists(ctx context.Context, provider, eventID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := provider + ":" + eventID
	_, exists := m.events[key]
	return exists, nil
}

func TestProcessWebhook_Success(t *testing.T) {
	mockProvider := payment.NewMockProvider("http://localhost:8080")
	eventRepo := newMockEventRepo()
	orderSvc := newMockOrderService()

	orderSvc.orders["ORD-WH-01"] = &domain.Order{
		ID:        1,
		Reference: "ORD-WH-01",
		Status:    domain.StatusPending,
	}

	service := payment.NewService(mockProvider, eventRepo, orderSvc)

	payload := map[string]string{
		"event_id":        "evt-001",
		"order_reference": "ORD-WH-01",
		"status":          "paid",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/mock", bytes.NewReader(body))

	err := service.ProcessWebhook(context.Background(), "mock", req)
	if err != nil {
		t.Fatalf("unexpected error processing webhook: %v", err)
	}

	// Verify order status is paid
	ord, _ := orderSvc.GetOrderByReference(context.Background(), "ORD-WH-01")
	if ord.Status != domain.StatusPaid {
		t.Errorf("expected order status paid, got %s", ord.Status)
	}

	// Verify event was recorded
	exists, _ := eventRepo.Exists(context.Background(), "mock", "evt-001")
	if !exists {
		t.Error("expected payment event to be recorded, but it was not")
	}
}

func TestProcessWebhook_Idempotency(t *testing.T) {
	mockProvider := payment.NewMockProvider("http://localhost:8080")
	eventRepo := newMockEventRepo()
	orderSvc := newMockOrderService()

	orderSvc.orders["ORD-WH-02"] = &domain.Order{
		ID:        2,
		Reference: "ORD-WH-02",
		Status:    domain.StatusPending,
	}

	service := payment.NewService(mockProvider, eventRepo, orderSvc)

	payload := map[string]string{
		"event_id":        "evt-002",
		"order_reference": "ORD-WH-02",
		"status":          "paid",
	}
	body, _ := json.Marshal(payload)

	// First call
	req1 := httptest.NewRequest(http.MethodPost, "/webhooks/mock", bytes.NewReader(body))
	if err := service.ProcessWebhook(context.Background(), "mock", req1); err != nil {
		t.Fatalf("first webhook failed: %v", err)
	}

	if orderSvc.updateCounts[2] != 1 {
		t.Fatalf("expected 1 update count, got %d", orderSvc.updateCounts[2])
	}

	// Second call (replay)
	req2 := httptest.NewRequest(http.MethodPost, "/webhooks/mock", bytes.NewReader(body))
	if err := service.ProcessWebhook(context.Background(), "mock", req2); err != nil {
		t.Fatalf("second webhook replay failed: %v", err)
	}

	// Must NOT increment update count again (idempotent no-op)
	if orderSvc.updateCounts[2] != 1 {
		t.Errorf("expected still 1 update count, but got %d", orderSvc.updateCounts[2])
	}
}

func TestProcessWebhook_MismatchedProvider(t *testing.T) {
	mockProvider := payment.NewMockProvider("http://localhost:8080")
	eventRepo := newMockEventRepo()
	orderSvc := newMockOrderService()

	service := payment.NewService(mockProvider, eventRepo, orderSvc)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/unknown", nil)
	err := service.ProcessWebhook(context.Background(), "unknown", req)
	if err == nil {
		t.Fatal("expected error for mismatched provider, got nil")
	}
}

func TestProcessWebhook_OrderNotFound(t *testing.T) {
	mockProvider := payment.NewMockProvider("http://localhost:8080")
	eventRepo := newMockEventRepo()
	orderSvc := newMockOrderService()

	service := payment.NewService(mockProvider, eventRepo, orderSvc)

	payload := map[string]string{
		"event_id":        "evt-999",
		"order_reference": "ORD-NONEXISTENT",
		"status":          "paid",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/mock", bytes.NewReader(body))

	err := service.ProcessWebhook(context.Background(), "mock", req)
	if err == nil {
		t.Fatal("expected error for non-existent order, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) && err.Error() == "" {
		t.Errorf("unexpected error format: %v", err)
	}
}

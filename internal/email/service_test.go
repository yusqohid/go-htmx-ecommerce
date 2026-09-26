package email_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/email"
)

func TestSendOrderReceipt_Success(t *testing.T) {
	mockSender := email.NewMockSender()
	service := email.NewService(mockSender, "https://store.example.com")
	ctx := context.Background()

	order := &domain.Order{
		Reference:       "ORD-20260926-TEST",
		CustomerID:      10,
		Status:          domain.StatusPaid,
		TotalAmount:     250000,
		Currency:        "IDR",
		PaymentProvider: "midtrans",
		CreatedAt:       time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
		Customer: &domain.User{
			ID:    10,
			Name:  "Jane Developer",
			Email: "jane@example.com",
		},
		Items: []domain.OrderItem{
			{
				ID:          1,
				ProductID:   5,
				ProductName: "Golang Microservices Masterclass",
				Price:       250000,
			},
		},
	}

	err := service.SendOrderReceipt(ctx, order)
	if err != nil {
		t.Fatalf("expected send receipt to succeed, got %v", err)
	}

	msgs := mockSender.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message dispatched, got %d", len(msgs))
	}

	msg := msgs[0]
	if msg.To != "jane@example.com" {
		t.Errorf("expected recipient jane@example.com, got %s", msg.To)
	}

	if !strings.Contains(msg.Subject, "ORD-20260926-TEST") {
		t.Errorf("subject should contain order reference, got %s", msg.Subject)
	}

	// HTML body checks
	if !strings.Contains(msg.HTMLBody, "Golang Microservices Masterclass") {
		t.Errorf("HTML body missing product name")
	}
	if !strings.Contains(msg.HTMLBody, "Rp 250.000") {
		t.Errorf("HTML body missing formatted price")
	}
	if !strings.Contains(msg.HTMLBody, "https://store.example.com/orders/ORD-20260926-TEST/success") {
		t.Errorf("HTML body missing download CTA link")
	}

	// Text body checks
	if !strings.Contains(msg.TextBody, "ORD-20260926-TEST") {
		t.Errorf("Text body missing order reference")
	}
	if !strings.Contains(msg.TextBody, "Rp 250.000") {
		t.Errorf("Text body missing formatted price")
	}
}

func TestSendOrderReceipt_NilOrder(t *testing.T) {
	mockSender := email.NewMockSender()
	service := email.NewService(mockSender, "")
	ctx := context.Background()

	err := service.SendOrderReceipt(ctx, nil)
	if err == nil {
		t.Error("expected error for nil order, got nil")
	}
}

func TestSendOrderReceipt_MissingRecipient(t *testing.T) {
	mockSender := email.NewMockSender()
	service := email.NewService(mockSender, "")
	ctx := context.Background()

	// 1. Nil customer
	orderNoCust := &domain.Order{Reference: "ORD-01"}
	err := service.SendOrderReceipt(ctx, orderNoCust)
	if !errors.Is(err, email.ErrMissingRecipient) {
		t.Errorf("expected ErrMissingRecipient for nil customer, got %v", err)
	}

	// 2. Empty email
	orderEmptyEmail := &domain.Order{
		Reference: "ORD-02",
		Customer:  &domain.User{Email: "  "},
	}
	err = service.SendOrderReceipt(ctx, orderEmptyEmail)
	if !errors.Is(err, email.ErrMissingRecipient) {
		t.Errorf("expected ErrMissingRecipient for empty email, got %v", err)
	}
}

func TestSendOrderReceipt_SenderError(t *testing.T) {
	mockSender := email.NewMockSender()
	injectedErr := errors.New("network timeout")
	mockSender.SetError(injectedErr)

	service := email.NewService(mockSender, "")
	ctx := context.Background()

	order := &domain.Order{
		Reference: "ORD-ERR",
		Customer:  &domain.User{Name: "Tester", Email: "test@example.com"},
	}

	err := service.SendOrderReceipt(ctx, order)
	if !errors.Is(err, injectedErr) {
		t.Errorf("expected injected error, got %v", err)
	}
}

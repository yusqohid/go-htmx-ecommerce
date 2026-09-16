package payment_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/payment"
)

func TestNewProvider(t *testing.T) {
	// 1. Mock provider
	cfgMock := &config.Config{
		PaymentProvider: "mock",
		AppBaseURL:      "http://localhost:8080",
	}
	pMock, err := payment.NewProvider(cfgMock)
	if err != nil {
		t.Fatalf("unexpected error creating mock provider: %v", err)
	}
	if pMock.Name() != "mock" {
		t.Errorf("expected name 'mock', got %s", pMock.Name())
	}

	// 2. Lynk provider without API key should error
	cfgLynkNoKey := &config.Config{
		PaymentProvider: "lynk",
	}
	_, err = payment.NewProvider(cfgLynkNoKey)
	if err == nil {
		t.Error("expected error for missing LYNK_API_KEY, got nil")
	}

	// 3. Lynk provider with API key
	cfgLynk := &config.Config{
		PaymentProvider:   "lynk",
		LynkAPIKey:       "test-api-key",
		LynkWebhookSecret: "test-secret",
		LynkBaseURL:       "https://api.lynk.id",
		AppBaseURL:        "http://localhost:8080",
	}
	pLynk, err := payment.NewProvider(cfgLynk)
	if err != nil {
		t.Fatalf("unexpected error creating lynk provider: %v", err)
	}
	if pLynk.Name() != "lynk" {
		t.Errorf("expected name 'lynk', got %s", pLynk.Name())
	}
}

func TestMockProvider_CreateCheckoutAndWebhook(t *testing.T) {
	provider := payment.NewMockProvider("http://localhost:8080")

	order := &domain.Order{
		ID:          1,
		Reference:   "ORD-20260916-TEST",
		TotalAmount: 150000,
	}

	checkoutURL, err := provider.CreateCheckout(context.Background(), order)
	if err != nil {
		t.Fatalf("CreateCheckout failed: %v", err)
	}

	expectedURL := "http://localhost:8080/mock-checkout?ref=ORD-20260916-TEST"
	if checkoutURL != expectedURL {
		t.Errorf("got checkoutURL %s, want %s", checkoutURL, expectedURL)
	}

	// Test webhook parsing
	payload := map[string]string{
		"event_id":        "evt-123",
		"order_reference": "ORD-20260916-TEST",
		"status":          "paid",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mock", bytes.NewReader(body))
	event, err := provider.VerifyWebhook(req)
	if err != nil {
		t.Fatalf("VerifyWebhook failed: %v", err)
	}

	if event.OrderReference != "ORD-20260916-TEST" {
		t.Errorf("expected order reference ORD-20260916-TEST, got %s", event.OrderReference)
	}
	if event.Status != domain.StatusPaid {
		t.Errorf("expected status paid, got %s", event.Status)
	}
}

func TestLynkProvider_CreateCheckout(t *testing.T) {
	// Mock external LYNK.ID server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"success": true,
			"message": "Transaction created",
			"data": {
				"checkout_url": "https://pay.lynk.id/checkout/session-123",
				"transaction_id": "TX-999"
			}
		}`))
	}))
	defer server.Close()

	provider := payment.NewLynkProvider("valid-test-key", "secret-123", server.URL, "http://localhost:8080")

	order := &domain.Order{
		ID:          42,
		Reference:   "ORD-LYNK-01",
		TotalAmount: 200000,
		Customer: &domain.User{
			Name:  "Test Customer",
			Email: "customer@example.com",
		},
	}

	checkoutURL, err := provider.CreateCheckout(context.Background(), order)
	if err != nil {
		t.Fatalf("CreateCheckout failed: %v", err)
	}

	expected := "https://pay.lynk.id/checkout/session-123"
	if checkoutURL != expected {
		t.Errorf("got %s, want %s", checkoutURL, expected)
	}
}

func TestLynkProvider_VerifyWebhook(t *testing.T) {
	secret := "my-secret-key"
	provider := payment.NewLynkProvider("key", secret, "https://api.lynk.id", "http://localhost:8080")

	rawJSON := []byte(`{
		"event_id": "EVT-888",
		"event_type": "payment.success",
		"data": {
			"merchant_order_id": "ORD-LYNK-01",
			"transaction_id": "TX-999",
			"status": "PAID",
			"amount": 200000
		}
	}`)

	// Calculate HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawJSON)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lynk", bytes.NewReader(rawJSON))
	req.Header.Set("X-Lynk-Signature", signature)

	event, err := provider.VerifyWebhook(req)
	if err != nil {
		t.Fatalf("VerifyWebhook failed: %v", err)
	}

	if event.OrderReference != "ORD-LYNK-01" {
		t.Errorf("got %s, want ORD-LYNK-01", event.OrderReference)
	}
	if event.Status != domain.StatusPaid {
		t.Errorf("got status %s, want paid", event.Status)
	}

	// Test invalid signature
	reqBad := httptest.NewRequest(http.MethodPost, "/webhooks/lynk", bytes.NewReader(rawJSON))
	reqBad.Header.Set("X-Lynk-Signature", "invalid-signature")
	_, errBad := provider.VerifyWebhook(reqBad)
	if errBad == nil {
		t.Error("expected error with invalid signature, got nil")
	}

	// Test missing webhook secret configured on provider
	providerNoSecret := payment.NewLynkProvider("key", "", "https://api.lynk.id", "http://localhost:8080")
	reqNoSecret := httptest.NewRequest(http.MethodPost, "/webhooks/lynk", bytes.NewReader(rawJSON))
	reqNoSecret.Header.Set("X-Lynk-Signature", signature)
	_, errNoSecret := providerNoSecret.VerifyWebhook(reqNoSecret)
	if errNoSecret == nil {
		t.Error("expected error when webhook secret is empty, got nil")
	}
}

func TestMockProvider_VerifyWebhook_RejectsUnsupportedStatus(t *testing.T) {
	provider := payment.NewMockProvider("http://localhost:8080")

	payload := map[string]string{
		"event_id":        "evt-invalid",
		"order_reference": "ORD-INVALID",
		"status":          "something_unsupported",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mock", bytes.NewReader(body))
	_, err := provider.VerifyWebhook(req)
	if err == nil {
		t.Fatal("expected error for unsupported mock webhook status, got nil")
	}
}

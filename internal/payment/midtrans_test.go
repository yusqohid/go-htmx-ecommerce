package payment_test

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/payment"
)

func TestMidtransProvider_Name(t *testing.T) {
	p := payment.NewMidtransProvider("server-key", "client-key", "", "http://localhost:8080", false)
	if p.Name() != "midtrans" {
		t.Errorf("expected provider name 'midtrans', got %s", p.Name())
	}
	if p.ClientKey() != "client-key" {
		t.Errorf("expected client key 'client-key', got %s", p.ClientKey())
	}
}

func TestMidtransProvider_CreateCheckout_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		user, pass, ok := r.BasicAuth()
		if !ok || user != "test-server-key" || pass != "" {
			t.Errorf("unexpected basic auth: user=%q, pass=%q, ok=%v", user, pass, ok)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		txDetails, _ := payload["transaction_details"].(map[string]any)
		if txDetails["order_id"] != "ORD-MID-01" {
			t.Errorf("expected order_id ORD-MID-01, got %v", txDetails["order_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":        "snap-token-123",
			"redirect_url": "https://app.sandbox.midtrans.com/snap/v2/vtweb/snap-token-123",
		})
	}))
	defer mockServer.Close()

	p := payment.NewMidtransProvider("test-server-key", "test-client-key", mockServer.URL, "http://localhost:8080", false)

	order := &domain.Order{
		Reference:   "ORD-MID-01",
		TotalAmount: 150000,
		Customer: &domain.User{
			Name:  "Budi Santoso",
			Email: "budi@example.com",
		},
		Items: []domain.OrderItem{
			{ProductID: 1, ProductName: "E-Book Go", Price: 150000},
		},
	}

	checkoutURL, err := p.CreateCheckout(context.Background(), order)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedURL := "https://app.sandbox.midtrans.com/snap/v2/vtweb/snap-token-123"
	if checkoutURL != expectedURL {
		t.Errorf("expected checkout URL %s, got %s", expectedURL, checkoutURL)
	}
}

func TestMidtransProvider_CreateCheckout_ErrorResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error_messages": ["order_id already used"]}`))
	}))
	defer mockServer.Close()

	p := payment.NewMidtransProvider("server-key", "client-key", mockServer.URL, "http://localhost:8080", false)

	order := &domain.Order{
		Reference:   "ORD-DUP",
		TotalAmount: 50000,
	}

	_, err := p.CreateCheckout(context.Background(), order)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

func TestMidtransProvider_VerifyWebhook_Success(t *testing.T) {
	serverKey := "my-secret-midtrans-key"
	p := payment.NewMidtransProvider(serverKey, "client-key", "", "http://localhost:8080", false)

	orderID := "ORD-MID-02"
	statusCode := "200"
	grossAmount := "150000.00"

	// Calculate SHA-512 signature: SHA512(order_id + status_code + gross_amount + server_key)
	rawSig := orderID + statusCode + grossAmount + serverKey
	hasher := sha512.New()
	hasher.Write([]byte(rawSig))
	signature := hex.EncodeToString(hasher.Sum(nil))

	payload := map[string]string{
		"order_id":           orderID,
		"status_code":        statusCode,
		"gross_amount":       grossAmount,
		"signature_key":      signature,
		"transaction_status": "settlement",
		"fraud_status":       "accept",
		"transaction_id":     "tx-mid-999",
		"payment_type":       "qris",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/midtrans", bytes.NewReader(body))

	event, err := p.VerifyWebhook(req)
	if err != nil {
		t.Fatalf("expected valid webhook, got err: %v", err)
	}

	if event.Provider != "midtrans" {
		t.Errorf("expected provider 'midtrans', got %s", event.Provider)
	}
	if event.OrderReference != orderID {
		t.Errorf("expected order reference %s, got %s", orderID, event.OrderReference)
	}
	if event.Status != domain.StatusPaid {
		t.Errorf("expected status 'paid', got %s", event.Status)
	}
	if event.Amount != 150000 {
		t.Errorf("expected amount 150000, got %d", event.Amount)
	}
	if event.PaymentReference != "tx-mid-999" {
		t.Errorf("expected payment ref tx-mid-999, got %s", event.PaymentReference)
	}
}

func TestMidtransProvider_VerifyWebhook_StatusMappings(t *testing.T) {
	serverKey := "key"
	p := payment.NewMidtransProvider(serverKey, "client", "", "http://localhost:8080", false)

	cases := []struct {
		txStatus    string
		fraudStatus string
		wantStatus  domain.OrderStatus
	}{
		{"capture", "accept", domain.StatusPaid},
		{"capture", "challenge", domain.StatusPending},
		{"pending", "accept", domain.StatusPending},
		{"deny", "accept", domain.StatusFailed},
		{"expire", "accept", domain.StatusFailed},
		{"cancel", "accept", domain.StatusCancelled},
		{"refund", "accept", domain.StatusRefunded},
	}

	for _, tc := range cases {
		t.Run(tc.txStatus+"_"+tc.fraudStatus, func(t *testing.T) {
			rawSig := "ORD-1" + "200" + "1000.00" + serverKey
			hasher := sha512.New()
			hasher.Write([]byte(rawSig))
			sig := hex.EncodeToString(hasher.Sum(nil))

			payload := map[string]string{
				"order_id":           "ORD-1",
				"status_code":        "200",
				"gross_amount":       "1000.00",
				"signature_key":      sig,
				"transaction_status": tc.txStatus,
				"fraud_status":       tc.fraudStatus,
				"transaction_id":     "tx-1",
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/webhooks/midtrans", bytes.NewReader(body))

			event, err := p.VerifyWebhook(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if event.Status != tc.wantStatus {
				t.Errorf("status mapping for %s/%s: expected %s, got %s", tc.txStatus, tc.fraudStatus, tc.wantStatus, event.Status)
			}
		})
	}
}

func TestMidtransProvider_VerifyWebhook_TamperedSignature(t *testing.T) {
	p := payment.NewMidtransProvider("server-key", "client-key", "", "http://localhost:8080", false)

	payload := map[string]string{
		"order_id":           "ORD-FAKE",
		"status_code":        "200",
		"gross_amount":       "100000.00",
		"signature_key":      "fake-invalid-signature",
		"transaction_status": "settlement",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/midtrans", bytes.NewReader(body))

	_, err := p.VerifyWebhook(req)
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
}

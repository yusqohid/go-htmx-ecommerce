package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// LynkProvider implements domain.PaymentProvider for LYNK.ID payment gateway.
type LynkProvider struct {
	apiKey        string
	webhookSecret string
	baseURL       string
	appBaseURL    string
	httpClient    *http.Client
}

// NewLynkProvider constructs a new LynkProvider.
func NewLynkProvider(apiKey, webhookSecret, baseURL, appBaseURL string) *LynkProvider {
	if baseURL == "" {
		baseURL = "https://api.lynk.id"
	}
	return &LynkProvider{
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		baseURL:       strings.TrimRight(baseURL, "/"),
		appBaseURL:    strings.TrimRight(appBaseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *LynkProvider) Name() string {
	return "lynk"
}

// LynkCheckoutRequest represents the payload sent to LYNK.ID to generate a payment session.
type LynkCheckoutRequest struct {
	MerchantOrderID string            `json:"merchant_order_id"`
	Amount          int64             `json:"amount"`
	Currency        string            `json:"currency"`
	CustomerEmail   string            `json:"customer_email,omitempty"`
	CustomerName    string            `json:"customer_name,omitempty"`
	RedirectURL     string            `json:"redirect_url"`
	CallbackURL     string            `json:"callback_url"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// LynkCheckoutResponse represents the response returned by LYNK.ID.
type LynkCheckoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		PaymentURL    string `json:"payment_url"`
		CheckoutURL   string `json:"checkout_url"`
		TransactionID string `json:"transaction_id"`
	} `json:"data"`
}

// CreateCheckout initiates a checkout transaction with LYNK.ID and retrieves the payment URL.
func (p *LynkProvider) CreateCheckout(ctx context.Context, order *domain.Order) (string, error) {
	if order == nil {
		return "", errors.New("order cannot be nil")
	}

	customerEmail := ""
	customerName := ""
	if order.Customer != nil {
		customerEmail = order.Customer.Email
		customerName = order.Customer.Name
	}

	payload := LynkCheckoutRequest{
		MerchantOrderID: order.Reference,
		Amount:          order.TotalAmount,
		Currency:        "IDR",
		CustomerEmail:   customerEmail,
		CustomerName:    customerName,
		RedirectURL:     fmt.Sprintf("%s/orders/%s/success", p.appBaseURL, order.Reference),
		CallbackURL:     fmt.Sprintf("%s/webhooks/lynk", p.appBaseURL),
		Metadata: map[string]string{
			"order_id": fmt.Sprintf("%d", order.ID),
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode checkout payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/transactions", bytes.NewReader(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call LYNK.ID api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read LYNK.ID response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("LYNK.ID api returned status %d: %s", resp.StatusCode, string(body))
	}

	var res LynkCheckoutResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed to parse LYNK.ID response: %w", err)
	}

	checkoutURL := res.Data.CheckoutURL
	if checkoutURL == "" {
		checkoutURL = res.Data.PaymentURL
	}

	if checkoutURL == "" {
		return "", errors.New("LYNK.ID response did not contain checkout or payment url")
	}

	return checkoutURL, nil
}

// VerifyWebhook validates and parses an incoming webhook event from LYNK.ID.
func (p *LynkProvider) VerifyWebhook(r *http.Request) (*domain.WebhookEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read webhook body: %w", err)
	}

	// Webhook secret is mandatory — reject requests if not configured.
	if p.webhookSecret == "" {
		return nil, errors.New("webhook secret is not configured; cannot verify webhook authenticity")
	}

	signature := r.Header.Get("X-Lynk-Signature")
	if signature == "" {
		return nil, errors.New("missing X-Lynk-Signature header")
	}

	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, errors.New("invalid webhook signature")
	}

	var payload struct {
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
		Data      struct {
			MerchantOrderID string `json:"merchant_order_id"`
			TransactionID   string `json:"transaction_id"`
			Status          string `json:"status"`
			Amount          int64  `json:"amount"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook json: %w", err)
	}

	status := domain.StatusPending
	switch strings.ToUpper(payload.Data.Status) {
	case "PAID", "SUCCESS", "SETTLED":
		status = domain.StatusPaid
	case "FAILED", "EXPIRED":
		status = domain.StatusFailed
	case "CANCELLED":
		status = domain.StatusCancelled
	case "REFUNDED":
		status = domain.StatusRefunded
	}

	eventID := payload.EventID
	if eventID == "" {
		eventID = payload.Data.TransactionID
	}

	return &domain.WebhookEvent{
		Provider:         p.Name(),
		EventID:          eventID,
		EventType:        payload.EventType,
		OrderReference:   payload.Data.MerchantOrderID,
		Status:           status,
		PaymentReference: payload.Data.TransactionID,
		RawPayload:       body,
	}, nil
}

package payment

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// MidtransProvider implements domain.PaymentProvider for Midtrans Snap payment gateway.
type MidtransProvider struct {
	serverKey    string
	clientKey    string
	snapURL      string
	apiBaseURL   string
	appBaseURL   string
	isProduction bool
	httpClient   *http.Client
}

// NewMidtransProvider constructs a new MidtransProvider.
func NewMidtransProvider(serverKey, clientKey, snapURL, appBaseURL string, isProduction bool) *MidtransProvider {
	apiBaseURL := "https://api.sandbox.midtrans.com"
	if snapURL == "" {
		if isProduction {
			snapURL = "https://app.midtrans.com/snap/v1/transactions"
			apiBaseURL = "https://api.midtrans.com"
		} else {
			snapURL = "https://app.sandbox.midtrans.com/snap/v1/transactions"
		}
	} else if isProduction {
		apiBaseURL = "https://api.midtrans.com"
	}

	return &MidtransProvider{
		serverKey:    strings.TrimSpace(serverKey),
		clientKey:    strings.TrimSpace(clientKey),
		snapURL:      snapURL,
		apiBaseURL:   apiBaseURL,
		appBaseURL:   strings.TrimRight(appBaseURL, "/"),
		isProduction: isProduction,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *MidtransProvider) Name() string {
	return "midtrans"
}

// ClientKey returns the Midtrans client key (useful for frontend scripts).
func (p *MidtransProvider) ClientKey() string {
	return p.clientKey
}

// SetAPIBaseURL allows overriding the base API URL (useful for test servers).
func (p *MidtransProvider) SetAPIBaseURL(url string) {
	p.apiBaseURL = strings.TrimRight(url, "/")
}

type midtransTransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

type midtransCustomerDetails struct {
	FirstName string `json:"first_name"`
	Email     string `json:"email"`
}

type midtransItemDetail struct {
	ID       string `json:"id"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
}

type midtransCallbacks struct {
	Finish string `json:"finish,omitempty"`
}

type midtransSnapRequest struct {
	TransactionDetails midtransTransactionDetails `json:"transaction_details"`
	CustomerDetails    *midtransCustomerDetails   `json:"customer_details,omitempty"`
	ItemDetails        []midtransItemDetail       `json:"item_details,omitempty"`
	Callbacks          *midtransCallbacks         `json:"callbacks,omitempty"`
}

type midtransSnapResponse struct {
	Token         string   `json:"token"`
	RedirectURL   string   `json:"redirect_url"`
	ErrorMessages []string `json:"error_messages,omitempty"`
}

// CreateCheckout initiates a transaction with Midtrans Snap and retrieves the redirect checkout URL.
func (p *MidtransProvider) CreateCheckout(ctx context.Context, order *domain.Order) (string, error) {
	if order == nil {
		return "", errors.New("order cannot be nil")
	}

	customerName := ""
	customerEmail := ""
	if order.Customer != nil {
		customerName = order.Customer.Name
		customerEmail = order.Customer.Email
	}

	var items []midtransItemDetail
	for _, item := range order.Items {
		items = append(items, midtransItemDetail{
			ID:       fmt.Sprintf("%d", item.ProductID),
			Price:    item.Price,
			Quantity: 1,
			Name:     item.ProductName,
		})
	}

	payload := midtransSnapRequest{
		TransactionDetails: midtransTransactionDetails{
			OrderID:     order.Reference,
			GrossAmount: order.TotalAmount,
		},
		CustomerDetails: &midtransCustomerDetails{
			FirstName: customerName,
			Email:     customerEmail,
		},
		ItemDetails: items,
		Callbacks: &midtransCallbacks{
			Finish: fmt.Sprintf("%s/orders/%s/success", p.appBaseURL, order.Reference),
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode Midtrans Snap payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.snapURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Midtrans expects Basic Auth with ServerKey as username and empty password
	auth := base64.StdEncoding.EncodeToString([]byte(p.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Midtrans Snap API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read Midtrans Snap response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("midtrans API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var res midtransSnapResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed to parse Midtrans Snap response: %w", err)
	}

	if res.RedirectURL == "" {
		return "", errors.New("midtrans response did not contain redirect_url")
	}

	return res.RedirectURL, nil
}

type midtransNotification struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	TransactionID     string `json:"transaction_id"`
	PaymentType       string `json:"payment_type"`
}

// VerifyWebhook validates and parses an incoming notification webhook from Midtrans.
func (p *MidtransProvider) VerifyWebhook(r *http.Request) (*domain.WebhookEvent, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB maximum limit
	if err != nil {
		return nil, fmt.Errorf("failed to read webhook body: %w", err)
	}

	if p.serverKey == "" {
		return nil, errors.New("midtrans server key is not configured; cannot verify webhook signature")
	}

	var payload midtransNotification
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid midtrans webhook JSON payload: %w", err)
	}
	return p.processNotificationPayload(payload, body)
}

// CheckStatus queries Midtrans Transaction Status API to directly verify the current payment status.
func (p *MidtransProvider) CheckStatus(ctx context.Context, orderReference string) (*domain.WebhookEvent, error) {
	if strings.TrimSpace(orderReference) == "" {
		return nil, errors.New("order reference cannot be empty")
	}

	url := fmt.Sprintf("%s/v2/%s/status", p.apiBaseURL, orderReference)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create status request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte(p.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Midtrans Status API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Transaction not yet registered in Midtrans
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read Midtrans status response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("midtrans Status API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var payload midtransNotification
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse Midtrans status JSON: %w", err)
	}

	return p.processNotificationPayload(payload, body)
}

func (p *MidtransProvider) processNotificationPayload(payload midtransNotification, body []byte) (*domain.WebhookEvent, error) {
	if payload.OrderID == "" || payload.StatusCode == "" || payload.GrossAmount == "" || payload.SignatureKey == "" {
		return nil, errors.New("incomplete midtrans notification fields for signature verification")
	}

	// Midtrans SHA512 signature formula: SHA512(order_id + status_code + gross_amount + ServerKey)
	rawSig := payload.OrderID + payload.StatusCode + payload.GrossAmount + p.serverKey
	hasher := sha512.New()
	hasher.Write([]byte(rawSig))
	expectedSignature := hex.EncodeToString(hasher.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(payload.SignatureKey), []byte(expectedSignature)) != 1 {
		return nil, errors.New("invalid midtrans webhook signature")
	}

	// Map Midtrans transaction status to domain OrderStatus
	status := domain.StatusPending
	switch strings.ToLower(payload.TransactionStatus) {
	case "capture":
		if strings.ToLower(payload.FraudStatus) == "challenge" {
			status = domain.StatusPending
		} else {
			status = domain.StatusPaid
		}
	case "settlement":
		status = domain.StatusPaid
	case "pending":
		status = domain.StatusPending
	case "deny", "expire":
		status = domain.StatusFailed
	case "cancel":
		status = domain.StatusCancelled
	case "refund", "partial_refund":
		status = domain.StatusRefunded
	}

	// Parse gross amount into integer cents/IDR
	var amountInt int64
	if f, err := strconv.ParseFloat(payload.GrossAmount, 64); err == nil {
		amountInt = int64(math.Round(f))
	}

	eventID := payload.TransactionID
	if eventID == "" {
		eventID = fmt.Sprintf("%s-%s", payload.OrderID, payload.StatusCode)
	}

	return &domain.WebhookEvent{
		Provider:         p.Name(),
		EventID:          eventID,
		EventType:        "midtrans." + payload.TransactionStatus,
		OrderReference:   payload.OrderID,
		Status:           status,
		PaymentReference: payload.TransactionID,
		Amount:           amountInt,
		RawPayload:       body,
	}, nil
}

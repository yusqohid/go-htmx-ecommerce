package order

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

var (
	// ErrAlreadyPurchased is returned when a customer attempts to repurchase a digital good they already own.
	ErrAlreadyPurchased = errors.New("you already own this digital product")
	// ErrProductNotAvailable is returned when attempting to order an unpublished or missing product.
	ErrProductNotAvailable = errors.New("product is not available for purchase")
)

// Service contains all order domain business rules and payment coordination.
type Service struct {
	orderRepo       domain.OrderRepository
	productRepo     domain.ProductRepository
	paymentProvider domain.PaymentProvider
}

// NewService constructs a new order Service.
func NewService(
	orderRepo domain.OrderRepository,
	productRepo domain.ProductRepository,
	paymentProvider domain.PaymentProvider,
) *Service {
	return &Service{
		orderRepo:       orderRepo,
		productRepo:     productRepo,
		paymentProvider: paymentProvider,
	}
}

// CreateOrderInput holds the parameters required to place an order.
type CreateOrderInput struct {
	Customer  *domain.User
	ProductID int64
}

// CreateOrder places a new pending order and initiates a payment checkout session.
func (s *Service) CreateOrder(ctx context.Context, input CreateOrderInput) (*domain.Order, string, error) {
	if input.Customer == nil {
		return nil, "", domain.ErrUnauthorized
	}

	// 1. Verify product exists and is currently published
	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, "", ErrProductNotAvailable
		}
		return nil, "", err
	}

	if !product.IsPublished() {
		return nil, "", ErrProductNotAvailable
	}

	// 2. Prevent duplicate purchases: check if customer already owns this product
	alreadyOwns, err := s.orderRepo.HasUserPurchasedProduct(ctx, input.Customer.ID, product.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to verify product ownership: %w", err)
	}
	if alreadyOwns {
		return nil, "", ErrAlreadyPurchased
	}

	// 3. Generate unique order reference: ORD-<YYYYMMDD>-<HEX>
	ref := GenerateOrderReference()

	// 4. Construct order with item snapshot (preserving historical price)
	order := &domain.Order{
		Reference:       ref,
		CustomerID:      input.Customer.ID,
		Customer:        input.Customer,
		Status:          domain.StatusPending,
		TotalAmount:     product.Price,
		Currency:        "IDR",
		PaymentProvider: s.paymentProvider.Name(),
		Items: []domain.OrderItem{
			{
				ProductID:   product.ID,
				ProductName: product.Name,
				Price:       product.Price,
			},
		},
	}

	// 5. Persist order atomically in database
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, "", fmt.Errorf("failed to save order: %w", err)
	}

	// 6. Request payment checkout URL from external/mock provider
	checkoutURL, err := s.paymentProvider.CreateCheckout(ctx, order)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create payment checkout: %w", err)
	}

	return order, checkoutURL, nil
}

// GetOrder retrieves an order by its ID, enforcing customer ownership authorization.
func (s *Service) GetOrder(ctx context.Context, id int64, requestingUserID int64, isAdmin bool) (*domain.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !isAdmin && order.CustomerID != requestingUserID {
		return nil, domain.ErrUnauthorized
	}

	return order, nil
}

// GetOrderByReference retrieves an order by its public reference code.
func (s *Service) GetOrderByReference(ctx context.Context, ref string) (*domain.Order, error) {
	return s.orderRepo.FindByReference(ctx, ref)
}

// ListCustomerOrders retrieves all past orders belonging to a customer.
func (s *Service) ListCustomerOrders(ctx context.Context, customerID int64) ([]domain.Order, error) {
	return s.orderRepo.ListByCustomerID(ctx, customerID)
}

// ListAllOrders retrieves paginated orders for administrative management.
func (s *Service) ListAllOrders(ctx context.Context, page, pageSize int) ([]domain.Order, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.orderRepo.ListAll(ctx, pageSize, offset)
}

// UpdateOrderStatus transitions an order to a new state with strict state machine validation.
func (s *Service) UpdateOrderStatus(ctx context.Context, id int64, status domain.OrderStatus, paymentRef string) error {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if order.Status == status {
		return nil
	}

	switch order.Status {
	case domain.StatusPending:
		if status != domain.StatusPaid && status != domain.StatusFailed && status != domain.StatusCancelled {
			return fmt.Errorf("invalid transition from pending to %s", status)
		}
	case domain.StatusPaid:
		if status != domain.StatusRefunded {
			return fmt.Errorf("%w: cannot transition paid order to %s", domain.ErrOrderAlreadyPaid, status)
		}
	default:
		return fmt.Errorf("cannot transition order in terminal state %s to %s", order.Status, status)
	}

	return s.orderRepo.UpdateStatus(ctx, id, status, paymentRef)
}

// ProcessPaymentResult atomically updates order status and records the payment event.
func (s *Service) ProcessPaymentResult(ctx context.Context, orderID int64, status domain.OrderStatus, paymentRef string, event *domain.PaymentEvent) error {
	return s.orderRepo.ProcessPaymentResult(ctx, orderID, status, paymentRef, event)
}
// SyncPaymentStatus queries the payment provider directly to synchronize order status if pending.
func (s *Service) SyncPaymentStatus(ctx context.Context, orderReference string) (*domain.Order, error) {
	ord, err := s.orderRepo.FindByReference(ctx, orderReference)
	if err != nil {
		return nil, err
	}

	if ord.Status == domain.StatusPaid || ord.Status == domain.StatusRefunded {
		return ord, nil
	}

	event, err := s.paymentProvider.CheckStatus(ctx, orderReference)
	if err != nil || event == nil {
		return ord, nil
	}

	if event.Status == domain.StatusPaid && event.Amount > 0 {
		if event.Amount != ord.TotalAmount {
			return ord, fmt.Errorf("payment amount mismatch: expected %d, got %d", ord.TotalAmount, event.Amount)
		}
	}

	pe := &domain.PaymentEvent{
		Provider:       event.Provider,
		EventID:        event.EventID,
		EventType:      event.EventType,
		OrderReference: event.OrderReference,
		Payload:        event.RawPayload,
		ProcessedAt:    time.Now().UTC(),
	}

	if err := s.orderRepo.ProcessPaymentResult(ctx, ord.ID, event.Status, event.PaymentReference, pe); err != nil {
		return ord, fmt.Errorf("failed to process payment sync: %w", err)
	}

	return s.orderRepo.FindByReference(ctx, orderReference)
}

// HasAccessToProduct checks whether the user owns a paid order for the given product.
func (s *Service) HasAccessToProduct(ctx context.Context, userID, productID int64) (bool, error) {
	return s.orderRepo.HasUserPurchasedProduct(ctx, userID, productID)
}

// GenerateOrderReference generates a unique human-friendly order identifier.
func GenerateOrderReference() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	timestamp := time.Now().UTC().Format("20060102")
	return fmt.Sprintf("ORD-%s-%s", timestamp, hex.EncodeToString(b))
}

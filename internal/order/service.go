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

// UpdateOrderStatus transitions an order to a new state (e.g. paid, failed, cancelled).
func (s *Service) UpdateOrderStatus(ctx context.Context, id int64, status domain.OrderStatus, paymentRef string) error {
	switch status {
	case domain.StatusPaid, domain.StatusFailed, domain.StatusCancelled, domain.StatusRefunded:
		return s.orderRepo.UpdateStatus(ctx, id, status, paymentRef)
	default:
		return fmt.Errorf("invalid order status transition to: %s", status)
	}
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

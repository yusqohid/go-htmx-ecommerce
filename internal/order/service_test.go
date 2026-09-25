package order_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/order"
)

// MockOrderRepo implements in-memory domain.OrderRepository.
type MockOrderRepo struct {
	orders      map[int64]*domain.Order
	byReference map[string]*domain.Order
	nextID      int64
}

func NewMockOrderRepo() *MockOrderRepo {
	return &MockOrderRepo{
		orders:      make(map[int64]*domain.Order),
		byReference: make(map[string]*domain.Order),
		nextID:      1,
	}
}

func (m *MockOrderRepo) Create(ctx context.Context, o *domain.Order) error {
	o.ID = m.nextID
	m.nextID++
	o.CreatedAt = time.Now()
	o.UpdatedAt = time.Now()
	m.orders[o.ID] = o
	m.byReference[o.Reference] = o
	return nil
}

func (m *MockOrderRepo) FindByID(ctx context.Context, id int64) (*domain.Order, error) {
	o, ok := m.orders[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return o, nil
}

func (m *MockOrderRepo) FindByReference(ctx context.Context, reference string) (*domain.Order, error) {
	o, ok := m.byReference[reference]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return o, nil
}

func (m *MockOrderRepo) ListByCustomerID(ctx context.Context, customerID int64) ([]domain.Order, error) {
	var res []domain.Order
	for _, o := range m.orders {
		if o.CustomerID == customerID {
			res = append(res, *o)
		}
	}
	return res, nil
}

func (m *MockOrderRepo) ListAll(ctx context.Context, limit, offset int) ([]domain.Order, int, error) {
	var res []domain.Order
	for _, o := range m.orders {
		res = append(res, *o)
	}
	return res, len(res), nil
}

func (m *MockOrderRepo) UpdateStatus(ctx context.Context, id int64, status domain.OrderStatus, paymentRef string) error {
	o, ok := m.orders[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.Status = status
	o.PaymentReference = paymentRef
	o.UpdatedAt = time.Now()
	return nil
}
func (m *MockOrderRepo) ProcessPaymentResult(ctx context.Context, id int64, status domain.OrderStatus, paymentRef string, event *domain.PaymentEvent) error {
	o, ok := m.orders[id]
	if !ok {
		return domain.ErrNotFound
	}
	if o.Status == domain.StatusPaid && status != domain.StatusRefunded && status != domain.StatusPaid {
		return domain.ErrOrderAlreadyPaid
	}
	o.Status = status
	o.PaymentReference = paymentRef
	o.UpdatedAt = time.Now()
	return nil
}

func (m *MockOrderRepo) HasUserPurchasedProduct(ctx context.Context, userID, productID int64) (bool, error) {
	for _, o := range m.orders {
		if o.CustomerID == userID && o.Status == domain.StatusPaid {
			for _, item := range o.Items {
				if item.ProductID == productID {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// MockProductRepo implements in-memory domain.ProductRepository.
type MockProductRepo struct {
	products map[int64]*domain.Product
}

func NewMockProductRepo() *MockProductRepo {
	return &MockProductRepo{products: make(map[int64]*domain.Product)}
}

func (m *MockProductRepo) Create(ctx context.Context, p *domain.Product) error {
	m.products[p.ID] = p
	return nil
}

func (m *MockProductRepo) FindByID(ctx context.Context, id int64) (*domain.Product, error) {
	p, ok := m.products[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (m *MockProductRepo) FindBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	for _, p := range m.products {
		if p.Slug == slug {
			return p, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockProductRepo) ListPublished(ctx context.Context, search string, limit, offset int) ([]domain.Product, int, error) {
	return nil, 0, nil
}

func (m *MockProductRepo) ListAll(ctx context.Context, search, status string, limit, offset int) ([]domain.Product, int, error) {
	return nil, 0, nil
}

func (m *MockProductRepo) Update(ctx context.Context, p *domain.Product) error {
	m.products[p.ID] = p
	return nil
}

func (m *MockProductRepo) Delete(ctx context.Context, id int64) error {
	delete(m.products, id)
	return nil
}

// MockPaymentProvider implements domain.PaymentProvider.
type MockPaymentProvider struct{}

func (m *MockPaymentProvider) Name() string { return "mock" }
func (m *MockPaymentProvider) CreateCheckout(ctx context.Context, order *domain.Order) (string, error) {
	return "https://mock.checkout.local/pay/" + order.Reference, nil
}
func (m *MockPaymentProvider) VerifyWebhook(r *http.Request) (*domain.WebhookEvent, error) {
	return nil, nil
}

func TestService_CreateOrder_Success(t *testing.T) {
	orderRepo := NewMockOrderRepo()
	productRepo := NewMockProductRepo()
	paymentProv := &MockPaymentProvider{}
	service := order.NewService(orderRepo, productRepo, paymentProv)
	ctx := context.Background()

	customer := &domain.User{ID: 10, Name: "Budi", Email: "budi@example.com"}
	product := &domain.Product{
		ID:     1,
		Name:   "Go Template",
		Price:  150000,
		Status: domain.StatusPublished,
	}
	_ = productRepo.Create(ctx, product)

	ord, checkoutURL, err := service.CreateOrder(ctx, order.CreateOrderInput{
		Customer:  customer,
		ProductID: product.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error creating order: %v", err)
	}

	if ord.TotalAmount != 150000 {
		t.Errorf("got total %d, want 150000", ord.TotalAmount)
	}
	if ord.Status != domain.StatusPending {
		t.Errorf("got status %s, want pending", ord.Status)
	}
	if len(ord.Items) != 1 || ord.Items[0].Price != 150000 {
		t.Errorf("expected 1 item with price snapshot 150000, got: %+v", ord.Items)
	}
	if checkoutURL != "https://mock.checkout.local/pay/"+ord.Reference {
		t.Errorf("unexpected checkoutURL: %s", checkoutURL)
	}
}

func TestService_CreateOrder_UnpublishedProduct(t *testing.T) {
	orderRepo := NewMockOrderRepo()
	productRepo := NewMockProductRepo()
	service := order.NewService(orderRepo, productRepo, &MockPaymentProvider{})
	ctx := context.Background()

	customer := &domain.User{ID: 10}
	draftProduct := &domain.Product{
		ID:     2,
		Name:   "Draft Course",
		Price:  200000,
		Status: domain.StatusDraft,
	}
	_ = productRepo.Create(ctx, draftProduct)

	_, _, err := service.CreateOrder(ctx, order.CreateOrderInput{
		Customer:  customer,
		ProductID: draftProduct.ID,
	})
	if !errors.Is(err, order.ErrProductNotAvailable) {
		t.Errorf("expected ErrProductNotAvailable, got %v", err)
	}
}

func TestService_CreateOrder_AlreadyPurchased(t *testing.T) {
	orderRepo := NewMockOrderRepo()
	productRepo := NewMockProductRepo()
	service := order.NewService(orderRepo, productRepo, &MockPaymentProvider{})
	ctx := context.Background()

	customer := &domain.User{ID: 10}
	prod := &domain.Product{ID: 3, Name: "Book", Price: 50000, Status: domain.StatusPublished}
	_ = productRepo.Create(ctx, prod)

	// Simulate existing paid order
	paidOrder := &domain.Order{
		Reference:   "ORD-PAID-01",
		CustomerID:  customer.ID,
		Status:      domain.StatusPaid,
		TotalAmount: 50000,
		Items: []domain.OrderItem{
			{ProductID: prod.ID, Price: 50000},
		},
	}
	_ = orderRepo.Create(ctx, paidOrder)

	// Attempting to buy again should return ErrAlreadyPurchased
	_, _, err := service.CreateOrder(ctx, order.CreateOrderInput{
		Customer:  customer,
		ProductID: prod.ID,
	})
	if !errors.Is(err, order.ErrAlreadyPurchased) {
		t.Errorf("expected ErrAlreadyPurchased, got %v", err)
	}
}

func TestService_GetOrder_Authorization(t *testing.T) {
	orderRepo := NewMockOrderRepo()
	service := order.NewService(orderRepo, NewMockProductRepo(), &MockPaymentProvider{})
	ctx := context.Background()

	ord := &domain.Order{
		Reference:   "ORD-AUTH-01",
		CustomerID:  10,
		Status:      domain.StatusPaid,
		TotalAmount: 100000,
	}
	_ = orderRepo.Create(ctx, ord)

	// Customer owner can view
	found, err := service.GetOrder(ctx, ord.ID, 10, false)
	if err != nil || found.ID != ord.ID {
		t.Errorf("customer owner should be able to view order, got err: %v", err)
	}

	// Another customer cannot view
	_, err = service.GetOrder(ctx, ord.ID, 99, false)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for other customer, got %v", err)
	}

	// Admin can view any order
	foundAdmin, err := service.GetOrder(ctx, ord.ID, 99, true)
	if err != nil || foundAdmin.ID != ord.ID {
		t.Errorf("admin should be able to view any order, got err: %v", err)
	}
}

func TestService_UpdateOrderStatus(t *testing.T) {
	orderRepo := NewMockOrderRepo()
	service := order.NewService(orderRepo, NewMockProductRepo(), &MockPaymentProvider{})
	ctx := context.Background()

	ord := &domain.Order{
		Reference:   "ORD-STATUS-01",
		CustomerID:  10,
		Status:      domain.StatusPending,
		TotalAmount: 100000,
	}
	_ = orderRepo.Create(ctx, ord)

	// Transition to paid
	err := service.UpdateOrderStatus(ctx, ord.ID, domain.StatusPaid, "TX-12345")
	if err != nil {
		t.Fatalf("failed to update status to paid: %v", err)
	}

	updated, _ := orderRepo.FindByID(ctx, ord.ID)
	if updated.Status != domain.StatusPaid || updated.PaymentReference != "TX-12345" {
		t.Errorf("unexpected status after update: %+v", updated)
	}

	// Invalid status should error
	err = service.UpdateOrderStatus(ctx, ord.ID, "unknown_status", "")
	if err == nil {
		t.Error("expected error for invalid status transition, got nil")
	}
}

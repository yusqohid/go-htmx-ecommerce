package customer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/customer"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

type mockOrderService struct {
	orders []domain.Order
}

func (m *mockOrderService) ListCustomerOrders(ctx context.Context, customerID int64) ([]domain.Order, error) {
	var result []domain.Order
	for _, o := range m.orders {
		if o.CustomerID == customerID {
			result = append(result, o)
		}
	}
	return result, nil
}

func (m *mockOrderService) GetOrderByReference(ctx context.Context, ref string) (*domain.Order, error) {
	for _, o := range m.orders {
		if o.Reference == ref {
			cpy := o
			return &cpy, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockOrderService) GetOrder(ctx context.Context, id int64, requestingUserID int64, isAdmin bool) (*domain.Order, error) {
	for _, o := range m.orders {
		if o.ID == id {
			if !isAdmin && o.CustomerID != requestingUserID {
				return nil, domain.ErrUnauthorized
			}
			cpy := o
			return &cpy, nil
		}
	}
	return nil, domain.ErrNotFound
}

type mockFileRepo struct {
	files map[int64][]domain.ProductFile
}

func (m *mockFileRepo) Create(ctx context.Context, file *domain.ProductFile) error {
	m.files[file.ProductID] = append(m.files[file.ProductID], *file)
	return nil
}

func (m *mockFileRepo) FindByID(ctx context.Context, id int64) (*domain.ProductFile, error) {
	for _, flist := range m.files {
		for _, f := range flist {
			if f.ID == id {
				return &f, nil
			}
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockFileRepo) FindByProductID(ctx context.Context, productID int64) ([]domain.ProductFile, error) {
	return m.files[productID], nil
}

func (m *mockFileRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func setupCustomerHandlerTest(t *testing.T) (*mockOrderService, *mockFileRepo, *customer.Handler) {
	t.Helper()

	orderSvc := &mockOrderService{}
	fileRepo := &mockFileRepo{files: make(map[int64][]domain.ProductFile)}

	mockFS := fstest.MapFS{
		"layouts/public.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/customer/orders.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>My Orders: {{ len .Orders }}</h1>{{ end }}`),
		},
		"pages/customer/order_detail.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Order {{ .Order.Reference }}</h1>{{ end }}`),
		},
	}

	renderer := view.New(mockFS, false)
	handler := customer.NewHandler(orderSvc, fileRepo, renderer)

	return orderSvc, fileRepo, handler
}

func TestCustomerHandler_AccountHome(t *testing.T) {
	_, _, handler := setupCustomerHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rec := httptest.NewRecorder()

	handler.AccountHome(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/account/orders" {
		t.Errorf("expected redirect to /account/orders, got %s", rec.Header().Get("Location"))
	}
}

func TestCustomerHandler_ListOrders(t *testing.T) {
	orderSvc, fileRepo, handler := setupCustomerHandlerTest(t)

	orderSvc.orders = []domain.Order{
		{
			ID:         1,
			Reference:  "ORD-CUST-01",
			CustomerID: 10,
			Status:     domain.StatusPaid,
			Items: []domain.OrderItem{
				{ProductID: 100, ProductName: "Digital Asset Pack"},
			},
		},
		{
			ID:         2,
			Reference:  "ORD-OTHER-01",
			CustomerID: 99,
			Status:     domain.StatusPaid,
		},
	}

	fileRepo.files[100] = []domain.ProductFile{
		{ID: 1, ProductID: 100, OriginalName: "asset.zip"},
	}

	r := chi.NewRouter()
	r.Get("/account/orders", handler.ListOrders)

	// 1. Unauthenticated -> redirect to /login
	reqUnauth := httptest.NewRequest(http.MethodGet, "/account/orders", nil)
	recUnauth := httptest.NewRecorder()
	r.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for unauth, got %d", recUnauth.Code)
	}

	// 2. Authenticated user 10 -> 200 OK
	user := &domain.User{ID: 10, Email: "cust@example.com"}
	reqAuth := httptest.NewRequest(http.MethodGet, "/account/orders", nil)
	reqAuth = reqAuth.WithContext(auth.WithUser(reqAuth.Context(), user))
	recAuth := httptest.NewRecorder()
	r.ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", recAuth.Code)
	}
}

func TestCustomerHandler_OrderDetail(t *testing.T) {
	orderSvc, _, handler := setupCustomerHandlerTest(t)

	orderSvc.orders = []domain.Order{
		{
			ID:         1,
			Reference:  "ORD-CUST-01",
			CustomerID: 10,
			Status:     domain.StatusPaid,
		},
	}

	r := chi.NewRouter()
	r.Get("/account/orders/{id}", handler.OrderDetail)

	// 1. Owner requests order -> 200 OK
	owner := &domain.User{ID: 10, Email: "owner@example.com"}
	reqOwner := httptest.NewRequest(http.MethodGet, "/account/orders/ORD-CUST-01", nil)
	reqOwner = reqOwner.WithContext(auth.WithUser(reqOwner.Context(), owner))
	recOwner := httptest.NewRecorder()
	r.ServeHTTP(recOwner, reqOwner)
	if recOwner.Code != http.StatusOK {
		t.Errorf("owner: expected 200 OK, got %d", recOwner.Code)
	}

	// 2. Other user requests order -> 403 Forbidden
	otherUser := &domain.User{ID: 99, Email: "other@example.com"}
	reqOther := httptest.NewRequest(http.MethodGet, "/account/orders/ORD-CUST-01", nil)
	reqOther = reqOther.WithContext(auth.WithUser(reqOther.Context(), otherUser))
	recOther := httptest.NewRecorder()
	r.ServeHTTP(recOther, reqOther)
	if recOther.Code != http.StatusForbidden {
		t.Errorf("other user: expected 403 Forbidden, got %d", recOther.Code)
	}
}

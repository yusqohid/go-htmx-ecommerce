package order_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/order"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

func setupAdminOrderHandlerTest(t *testing.T) (*order.Service, *MockOrderRepo, *order.AdminHandler) {
	t.Helper()

	orderRepo := NewMockOrderRepo()
	productRepo := NewMockProductRepo()
	service := order.NewService(orderRepo, productRepo, &MockPaymentProvider{})

	mockFS := fstest.MapFS{
		"layouts/admin.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/admin/orders/index.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Orders List: {{ len .Orders }}</h1>{{ end }}`),
		},
		"pages/admin/orders/detail.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Order Detail: {{ .Order.Reference }}</h1>{{ end }}`),
		},
	}

	renderer := view.New(mockFS, false)
	adminHandler := order.NewAdminHandler(service, renderer)

	return service, orderRepo, adminHandler
}

func TestAdminHandler_ListOrders(t *testing.T) {
	_, orderRepo, adminHandler := setupAdminOrderHandlerTest(t)
	ctx := context.Background()

	_ = orderRepo.Create(ctx, &domain.Order{
		Reference:   "ORD-ADM-01",
		CustomerID:  1,
		Status:      domain.StatusPaid,
		TotalAmount: 100000,
	})
	_ = orderRepo.Create(ctx, &domain.Order{
		Reference:   "ORD-ADM-02",
		CustomerID:  2,
		Status:      domain.StatusPending,
		TotalAmount: 50000,
	})

	r := chi.NewRouter()
	r.Get("/admin/orders", adminHandler.ListOrders)

	adminUser := &domain.User{ID: 1, Role: domain.RoleAdmin}
	req := httptest.NewRequest(http.MethodGet, "/admin/orders?page=1", nil)
	req = req.WithContext(auth.WithUser(req.Context(), adminUser))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}

func TestAdminHandler_OrderDetail(t *testing.T) {
	_, orderRepo, adminHandler := setupAdminOrderHandlerTest(t)
	ctx := context.Background()

	ord := &domain.Order{
		Reference:   "ORD-ADM-03",
		CustomerID:  1,
		Status:      domain.StatusPaid,
		TotalAmount: 120000,
	}
	_ = orderRepo.Create(ctx, ord)

	r := chi.NewRouter()
	r.Get("/admin/orders/{id}", adminHandler.OrderDetail)

	adminUser := &domain.User{ID: 1, Role: domain.RoleAdmin}

	// 1. Existing order -> 200 OK
	req := httptest.NewRequest(http.MethodGet, "/admin/orders/1", nil)
	req = req.WithContext(auth.WithUser(req.Context(), adminUser))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}

	// 2. Non-existent order -> redirect to /admin/orders
	reqMissing := httptest.NewRequest(http.MethodGet, "/admin/orders/999", nil)
	reqMissing = reqMissing.WithContext(auth.WithUser(reqMissing.Context(), adminUser))
	recMissing := httptest.NewRecorder()
	r.ServeHTTP(recMissing, reqMissing)

	if recMissing.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for missing order, got %d", recMissing.Code)
	}
}

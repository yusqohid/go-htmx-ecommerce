package order_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/order"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

func setupOrderHandlerTest(t *testing.T) (*order.Service, *MockProductRepo, *MockOrderRepo, *order.Handler) {
	t.Helper()

	orderRepo := NewMockOrderRepo()
	productRepo := NewMockProductRepo()
	service := order.NewService(orderRepo, productRepo, &MockPaymentProvider{})

	mockFS := fstest.MapFS{
		"layouts/public.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/storefront/checkout.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Checkout: {{ .Product.Name }}</h1>{{ end }}`),
		},
		"pages/storefront/order_success.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Order: {{ .Order.Reference }}</h1>{{ end }}`),
		},
		"pages/storefront/mock_checkout.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Mock Portal: {{ .Order.Reference }}</h1>{{ end }}`),
		},
	}

	renderer := view.New(mockFS, false)
	handler := order.NewHandler(service, productRepo, renderer, "mock")

	return service, productRepo, orderRepo, handler
}

func TestCheckoutPage_Unauthenticated(t *testing.T) {
	_, _, _, handler := setupOrderHandlerTest(t)

	r := chi.NewRouter()
	r.Get("/checkout/{productID}", handler.CheckoutPage)

	req := httptest.NewRequest(http.MethodGet, "/checkout/1", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Errorf("expected redirect to login, got %s", rec.Header().Get("Location"))
	}
}

func TestCheckoutPage_Authenticated(t *testing.T) {
	_, productRepo, _, handler := setupOrderHandlerTest(t)
	ctx := context.Background()

	prod := &domain.Product{ID: 1, Name: "Asset Pack", Price: 75000, Status: domain.StatusPublished}
	_ = productRepo.Create(ctx, prod)

	user := &domain.User{ID: 10, Name: "Customer A", Email: "customer@example.com"}

	r := chi.NewRouter()
	r.Get("/checkout/{productID}", handler.CheckoutPage)

	req := httptest.NewRequest(http.MethodGet, "/checkout/1", nil)
	// Inject authenticated user into context
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}

func TestProcessCheckout_CreatesOrderAndRedirects(t *testing.T) {
	_, productRepo, _, handler := setupOrderHandlerTest(t)
	ctx := context.Background()

	prod := &domain.Product{ID: 1, Name: "Asset Pack", Price: 75000, Status: domain.StatusPublished}
	_ = productRepo.Create(ctx, prod)

	user := &domain.User{ID: 10, Name: "Customer A", Email: "customer@example.com"}

	r := chi.NewRouter()
	r.Post("/checkout/{productID}", handler.ProcessCheckout)

	req := httptest.NewRequest(http.MethodPost, "/checkout/1", nil)
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "https://mock.checkout.local/pay/ORD-") {
		t.Errorf("expected redirect to checkoutURL, got %s", location)
	}
}

func TestMockSimulatePayment(t *testing.T) {
	_, _, orderRepo, handler := setupOrderHandlerTest(t)
	ctx := context.Background()

	ord := &domain.Order{
		Reference:   "ORD-SIM-01",
		CustomerID:  10,
		Status:      domain.StatusPending,
		TotalAmount: 75000,
	}
	_ = orderRepo.Create(ctx, ord)

	r := chi.NewRouter()
	r.Post("/mock-checkout/simulate", handler.MockSimulatePayment)

	form := url.Values{}
	form.Set("ref", ord.Reference)
	form.Set("status", "paid")

	req := httptest.NewRequest(http.MethodPost, "/mock-checkout/simulate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	expectedSuccessURL := "/orders/" + ord.Reference + "/success"
	if rec.Header().Get("Location") != expectedSuccessURL {
		t.Errorf("got redirect %s, want %s", rec.Header().Get("Location"), expectedSuccessURL)
	}

	// Verify order status updated in repo
	updated, _ := orderRepo.FindByReference(ctx, ord.Reference)
	if updated.Status != domain.StatusPaid {
		t.Errorf("expected order status paid, got %s", updated.Status)
	}
}

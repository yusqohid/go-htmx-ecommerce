package order_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/coupon"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/order"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

func setupOrderHandlerTest(t *testing.T) (*order.Service, *MockProductRepo, *MockOrderRepo, *order.Handler) {
	t.Helper()

	orderRepo := NewMockOrderRepo()
	productRepo := NewMockProductRepo()
	couponRepo := NewMockOrderCouponRepo()
	couponService := coupon.NewService(couponRepo)
	service := order.NewService(orderRepo, productRepo, couponRepo, &MockPaymentProvider{}, nil)

	mockFS := fstest.MapFS{
		"layouts/public.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/storefront/checkout.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Checkout: {{ .Product.Name }}</h1><input type="hidden" id="checkout-coupon-code" name="coupon_code" form="checkout-form" value="{{ if .Coupon }}{{ .Coupon.Code }}{{ else if .CouponCode }}{{ .CouponCode }}{{ end }}">{{ if .Coupon }}<span>Coupon: {{ .Coupon.Code }}</span>{{ end }}{{ if .CouponError }}<span class="error">{{ .CouponError }}</span>{{ end }}<span class="total">{{ .TotalAmount }}</span>{{ end }}`),
		},
		"pages/storefront/order_success.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Order: {{ .Order.Reference }}</h1>{{ end }}`),
		},
		"pages/storefront/mock_checkout.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Mock Portal: {{ .Order.Reference }}</h1>{{ end }}`),
		},
	}

	renderer := view.New(mockFS, false)
	handler := order.NewHandler(service, productRepo, couponService, renderer, "mock", "test-client-key", "https://app.sandbox.midtrans.com/snap/snap.js")

	return service, productRepo, orderRepo, handler
}

func setupOrderHandlerCouponTest(t *testing.T) (*order.Service, *MockProductRepo, *MockOrderRepo, *MockOrderCouponRepo, *order.Handler) {
	t.Helper()

	orderRepo := NewMockOrderRepo()
	productRepo := NewMockProductRepo()
	couponRepo := NewMockOrderCouponRepo()
	couponService := coupon.NewService(couponRepo)
	service := order.NewService(orderRepo, productRepo, couponRepo, &MockPaymentProvider{}, nil)

	mockFS := fstest.MapFS{
		"layouts/public.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/storefront/checkout.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Checkout: {{ .Product.Name }}</h1><input type="hidden" id="checkout-coupon-code" name="coupon_code" form="checkout-form" value="{{ if .Coupon }}{{ .Coupon.Code }}{{ else if .CouponCode }}{{ .CouponCode }}{{ end }}">{{ if .Coupon }}<span>Coupon: {{ .Coupon.Code }}</span>{{ end }}{{ if .CouponError }}<span class="error">{{ .CouponError }}</span>{{ end }}<span class="total">{{ .TotalAmount }}</span>{{ end }}`),
		},
		"pages/storefront/order_success.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Order: {{ .Order.Reference }}</h1>{{ end }}`),
		},
		"pages/storefront/mock_checkout.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Mock Portal: {{ .Order.Reference }}</h1>{{ end }}`),
		},
	}

	renderer := view.New(mockFS, false)
	handler := order.NewHandler(service, productRepo, couponService, renderer, "mock", "test-client-key", "https://app.sandbox.midtrans.com/snap/snap.js")

	return service, productRepo, orderRepo, couponRepo, handler
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

	user := &domain.User{ID: 10, Email: "customer@example.com"}
	req := httptest.NewRequest(http.MethodPost, "/mock-checkout/simulate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), user))
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

func TestMockSimulatePayment_ForbiddenForOtherUser(t *testing.T) {
	_, _, orderRepo, handler := setupOrderHandlerTest(t)
	ctx := context.Background()

	ord := &domain.Order{
		Reference:   "ORD-SIM-02",
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

	otherUser := &domain.User{ID: 99, Email: "attacker@example.com"}
	req := httptest.NewRequest(http.MethodPost, "/mock-checkout/simulate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), otherUser))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
	}
}

func TestMockSimulatePayment_InvalidStatus(t *testing.T) {
	_, _, orderRepo, handler := setupOrderHandlerTest(t)
	ctx := context.Background()

	ord := &domain.Order{
		Reference:   "ORD-SIM-03",
		CustomerID:  10,
		Status:      domain.StatusPending,
		TotalAmount: 75000,
	}
	_ = orderRepo.Create(ctx, ord)

	r := chi.NewRouter()
	r.Post("/mock-checkout/simulate", handler.MockSimulatePayment)

	form := url.Values{}
	form.Set("ref", ord.Reference)
	form.Set("status", "random_malicious_status")

	user := &domain.User{ID: 10, Email: "customer@example.com"}
	req := httptest.NewRequest(http.MethodPost, "/mock-checkout/simulate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestOrderSuccess_OwnershipEnforced(t *testing.T) {
	_, _, orderRepo, handler := setupOrderHandlerTest(t)
	ctx := context.Background()

	ord := &domain.Order{
		Reference:   "ORD-SUCC-01",
		CustomerID:  10,
		Status:      domain.StatusPaid,
		TotalAmount: 75000,
	}
	_ = orderRepo.Create(ctx, ord)

	r := chi.NewRouter()
	r.Get("/orders/{reference}/success", handler.OrderSuccess)

	// 1. Unauthenticated -> redirect to login
	reqUnauth := httptest.NewRequest(http.MethodGet, "/orders/ORD-SUCC-01/success", nil)
	recUnauth := httptest.NewRecorder()
	r.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusSeeOther {
		t.Errorf("unauth: expected 303 redirect, got %d", recUnauth.Code)
	}

	// 2. Authenticated but different customer -> 403 Forbidden
	attacker := &domain.User{ID: 99, Email: "attacker@example.com"}
	reqAttacker := httptest.NewRequest(http.MethodGet, "/orders/ORD-SUCC-01/success", nil)
	reqAttacker = reqAttacker.WithContext(auth.WithUser(reqAttacker.Context(), attacker))
	recAttacker := httptest.NewRecorder()
	r.ServeHTTP(recAttacker, reqAttacker)
	if recAttacker.Code != http.StatusForbidden {
		t.Errorf("other user: expected 403 Forbidden, got %d", recAttacker.Code)
	}

	// 3. Legitimate owner -> 200 OK
	owner := &domain.User{ID: 10, Email: "owner@example.com"}
	reqOwner := httptest.NewRequest(http.MethodGet, "/orders/ORD-SUCC-01/success", nil)
	reqOwner = reqOwner.WithContext(auth.WithUser(reqOwner.Context(), owner))
	recOwner := httptest.NewRecorder()
	r.ServeHTTP(recOwner, reqOwner)
	if recOwner.Code != http.StatusOK {
		t.Errorf("owner: expected 200 OK, got %d", recOwner.Code)
	}
}
func TestProcessCheckout_JSON(t *testing.T) {
	_, productRepo, _, handler := setupOrderHandlerTest(t)
	ctx := context.Background()

	prod := &domain.Product{
		Name:   "JSON Checkout Prod",
		Price:  100000,
		Status: domain.StatusPublished,
	}
	_ = productRepo.Create(ctx, prod)

	r := chi.NewRouter()
	r.Post("/checkout/{productID}", handler.ProcessCheckout)

	user := &domain.User{ID: 25, Email: "ajax@example.com"}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/checkout/%d", prod.ID), nil)
	req.Header.Set("Accept", "application/json")
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for JSON checkout, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if payload["order_reference"] == "" || payload["order_reference"] == nil {
		t.Error("expected non-empty order_reference")
	}
	if payload["checkout_url"] == "" || payload["checkout_url"] == nil {
		t.Error("expected non-empty checkout_url")
	}
}

func TestApplyCoupon_Success(t *testing.T) {
	_, productRepo, _, couponRepo, handler := setupOrderHandlerCouponTest(t)
	ctx := context.Background()

	prod := &domain.Product{
		Name:   "Ebook Go Advanced",
		Price:  100000,
		Status: domain.StatusPublished,
	}
	_ = productRepo.Create(ctx, prod)

	_ = couponRepo.Create(ctx, &domain.Coupon{
		ID:            10,
		Code:          "DISKON20",
		DiscountType:  domain.DiscountTypePercentage,
		DiscountValue: 20,
		IsActive:      true,
	})

	r := chi.NewRouter()
	r.Post("/checkout/{productID}/apply-coupon", handler.ApplyCoupon)

	user := &domain.User{ID: 30, Email: "user@example.com"}
	form := url.Values{}
	form.Set("coupon_code", "diskon20")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/checkout/%d/apply-coupon", prod.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "DISKON20") {
		t.Errorf("expected rendered response to contain coupon code DISKON20, got: %s", body)
	}
	if !strings.Contains(body, `form="checkout-form"`) {
		t.Errorf("expected rendered response to associate coupon input with form='checkout-form'")
	}
	if !strings.Contains(body, `value="DISKON20"`) {
		t.Errorf("expected rendered response to set coupon input value to DISKON20")
	}
	if !strings.Contains(body, "80000") {
		t.Errorf("expected rendered total to be 80000, got: %s", body)
	}
}

func TestApplyCoupon_InvalidCoupon(t *testing.T) {
	_, productRepo, _, _, handler := setupOrderHandlerCouponTest(t)
	ctx := context.Background()

	prod := &domain.Product{
		Name:   "Ebook Go Advanced",
		Price:  100000,
		Status: domain.StatusPublished,
	}
	_ = productRepo.Create(ctx, prod)

	r := chi.NewRouter()
	r.Post("/checkout/{productID}/apply-coupon", handler.ApplyCoupon)

	user := &domain.User{ID: 30, Email: "user@example.com"}
	form := url.Values{}
	form.Set("coupon_code", "EXPIREDCODE")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/checkout/%d/apply-coupon", prod.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Kode kupon tidak ditemukan") {
		t.Errorf("expected error message in response, got: %s", body)
	}
	if !strings.Contains(body, "100000") {
		t.Errorf("expected original total 100000, got: %s", body)
	}
}

func TestRemoveCoupon(t *testing.T) {
	_, productRepo, _, _, handler := setupOrderHandlerCouponTest(t)
	ctx := context.Background()

	prod := &domain.Product{
		Name:   "Ebook Go Advanced",
		Price:  100000,
		Status: domain.StatusPublished,
	}
	_ = productRepo.Create(ctx, prod)

	r := chi.NewRouter()
	r.Post("/checkout/{productID}/remove-coupon", handler.RemoveCoupon)

	user := &domain.User{ID: 30, Email: "user@example.com"}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/checkout/%d/remove-coupon", prod.ID), nil)
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "Coupon:") {
		t.Errorf("expected no coupon in body, got: %s", body)
	}
	if !strings.Contains(body, "100000") {
		t.Errorf("expected total 100000, got: %s", body)
	}
}

func TestCheckoutPage_WithCouponQuery(t *testing.T) {
	_, productRepo, _, couponRepo, handler := setupOrderHandlerCouponTest(t)
	ctx := context.Background()

	prod := &domain.Product{
		Name:   "Ebook Go Advanced",
		Price:  100000,
		Status: domain.StatusPublished,
	}
	_ = productRepo.Create(ctx, prod)

	_ = couponRepo.Create(ctx, &domain.Coupon{
		ID:            11,
		Code:          "HEMAT50",
		DiscountType:  domain.DiscountTypePercentage,
		DiscountValue: 50,
		IsActive:      true,
	})

	r := chi.NewRouter()
	r.Get("/checkout/{productID}", handler.CheckoutPage)

	user := &domain.User{ID: 30, Email: "user@example.com"}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/checkout/%d?coupon=hemat50", prod.ID), nil)
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "HEMAT50") {
		t.Errorf("expected rendered response to contain coupon code HEMAT50, got: %s", body)
	}
	if !strings.Contains(body, "50000") {
		t.Errorf("expected rendered total to be 50000, got: %s", body)
	}
}

func TestProcessCheckout_WithCoupon(t *testing.T) {
	_, productRepo, orderRepo, couponRepo, handler := setupOrderHandlerCouponTest(t)
	ctx := context.Background()

	prod := &domain.Product{
		Name:   "Course Go",
		Price:  200000,
		Status: domain.StatusPublished,
	}
	_ = productRepo.Create(ctx, prod)

	_ = couponRepo.Create(ctx, &domain.Coupon{
		ID:            12,
		Code:          "POTONGAN50K",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 50000,
		IsActive:      true,
	})

	r := chi.NewRouter()
	r.Post("/checkout/{productID}", handler.ProcessCheckout)

	user := &domain.User{ID: 35, Email: "customer@example.com"}
	form := url.Values{}
	form.Set("coupon_code", "potongan50k")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/checkout/%d", prod.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	orders, err := orderRepo.ListByCustomerID(ctx, user.ID)
	if err != nil || len(orders) != 1 {
		t.Fatalf("expected 1 order created, got %d (err: %v)", len(orders), err)
	}

	created := orders[0]
	if created.TotalAmount != 150000 {
		t.Errorf("expected total 150000 after 50000 discount, got %d", created.TotalAmount)
	}
	if created.DiscountAmount != 50000 {
		t.Errorf("expected discount 50000, got %d", created.DiscountAmount)
	}
	if created.CouponID == nil || *created.CouponID != 12 {
		t.Errorf("expected CouponID 12, got %v", created.CouponID)
	}
}

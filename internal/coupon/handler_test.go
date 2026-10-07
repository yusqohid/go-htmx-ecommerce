package coupon_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/coupon"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

func setupCouponHandlerTest(t *testing.T) (*coupon.Service, *MockCouponRepo, *coupon.Handler) {
	t.Helper()
	repo := NewMockCouponRepo()
	service := coupon.NewService(repo)

	mockFS := fstest.MapFS{
		"layouts/admin.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/admin/coupons/index.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Coupons: {{ len .Coupons }}</h1>{{ end }}`),
		},
		"pages/admin/coupons/form.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Form IsEdit={{ .IsEdit }} Error={{ .Error }}</h1>{{ end }}`),
		},
	}

	renderer := view.New(mockFS, false)
	handler := coupon.NewHandler(service, renderer)
	return service, repo, handler
}

func TestCouponHandler_ListCoupons(t *testing.T) {
	_, repo, handler := setupCouponHandlerTest(t)
	ctx := context.Background()

	_ = repo.Create(ctx, &domain.Coupon{
		Code:          "WELCOME",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 10000,
		IsActive:      true,
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/coupons", nil)
	rec := httptest.NewRecorder()

	handler.ListCoupons(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Coupons: 1") {
		t.Errorf("expected body to contain 'Coupons: 1', got %s", rec.Body.String())
	}
}

func TestCouponHandler_NewCoupon(t *testing.T) {
	_, _, handler := setupCouponHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/coupons/new", nil)
	rec := httptest.NewRecorder()

	handler.NewCoupon(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Form IsEdit=false") {
		t.Errorf("expected form in create mode, got %s", rec.Body.String())
	}
}

func TestCouponHandler_CreateCoupon(t *testing.T) {
	_, repo, handler := setupCouponHandlerTest(t)

	// 1. Success create
	formData := url.Values{
		"code":                {"PROMO25"},
		"discount_type":       {"percentage"},
		"discount_value":      {"25"},
		"min_purchase_amount": {"50000"},
		"max_discount_amount": {"20000"},
		"is_active":           {"on"},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/coupons", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.CreateCoupon(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 See Other redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/coupons" {
		t.Errorf("expected redirect to /admin/coupons, got %s", loc)
	}

	c, err := repo.FindByCode(context.Background(), "PROMO25")
	if err != nil || c.DiscountValue != 25 {
		t.Fatalf("failed to find created coupon: %v", err)
	}

	// 2. Validation error (empty code)
	invalidData := url.Values{
		"code":           {""},
		"discount_type":  {"percentage"},
		"discount_value": {"25"},
	}
	reqBad := httptest.NewRequest(http.MethodPost, "/admin/coupons", strings.NewReader(invalidData.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()

	handler.CreateCoupon(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", recBad.Code)
	}
}

func TestCouponHandler_EditAndUpdateCoupon(t *testing.T) {
	_, repo, handler := setupCouponHandlerTest(t)
	ctx := context.Background()

	c := &domain.Coupon{
		Code:          "EDITME",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 15000,
		IsActive:      true,
	}
	_ = repo.Create(ctx, c)

	// 1. Show Edit form
	r := chi.NewRouter()
	r.Get("/admin/coupons/{id}/edit", handler.EditCoupon)
	r.Post("/admin/coupons/{id}", handler.UpdateCoupon)

	reqEdit := httptest.NewRequest(http.MethodGet, "/admin/coupons/"+strconv.FormatInt(c.ID, 10)+"/edit", nil)
	recEdit := httptest.NewRecorder()
	r.ServeHTTP(recEdit, reqEdit)

	if recEdit.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recEdit.Code)
	}
	if !strings.Contains(recEdit.Body.String(), "Form IsEdit=true") {
		t.Errorf("expected form in edit mode, got %s", recEdit.Body.String())
	}

	// 2. Update coupon
	formData := url.Values{
		"code":           {"EDITED"},
		"discount_type":  {"fixed"},
		"discount_value": {"20000"},
		"is_active":      {"on"},
	}
	reqUpdate := httptest.NewRequest(http.MethodPost, "/admin/coupons/"+strconv.FormatInt(c.ID, 10), strings.NewReader(formData.Encode()))
	reqUpdate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recUpdate := httptest.NewRecorder()
	r.ServeHTTP(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", recUpdate.Code)
	}

	updated, _ := repo.FindByID(ctx, c.ID)
	if updated.Code != "EDITED" || updated.DiscountValue != 20000 {
		t.Errorf("unexpected updated coupon: %+v", updated)
	}
}

func TestCouponHandler_ToggleStatus(t *testing.T) {
	_, repo, handler := setupCouponHandlerTest(t)
	ctx := context.Background()

	c := &domain.Coupon{
		Code:          "TOGGLE",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 5000,
		IsActive:      true,
	}
	_ = repo.Create(ctx, c)

	r := chi.NewRouter()
	r.Post("/admin/coupons/{id}/toggle-status", handler.ToggleStatus)

	req := httptest.NewRequest(http.MethodPost, "/admin/coupons/"+strconv.FormatInt(c.ID, 10)+"/toggle-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Inactive") {
		t.Errorf("expected badge to be Inactive after toggle, got %s", rec.Body.String())
	}
}

func TestCouponHandler_DeleteCoupon(t *testing.T) {
	_, repo, handler := setupCouponHandlerTest(t)
	ctx := context.Background()

	c := &domain.Coupon{
		Code:          "DELETE",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 5000,
		IsActive:      true,
	}
	_ = repo.Create(ctx, c)

	r := chi.NewRouter()
	r.Post("/admin/coupons/{id}/delete", handler.DeleteCoupon)

	req := httptest.NewRequest(http.MethodPost, "/admin/coupons/"+strconv.FormatInt(c.ID, 10)+"/delete", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	_, err := repo.FindByID(ctx, c.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for deleted coupon, got %v", err)
	}
}

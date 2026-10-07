package view_test

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
	"github.com/yusqohid/go-htmx-ecommerce/web/templates"
)

func TestFuncMap(t *testing.T) {
	funcs := view.FuncMap()

	// formatMoney test
	formatMoney := funcs["formatMoney"].(func(int64) string)
	if got := formatMoney(150000); got != "Rp 150.000" {
		t.Errorf("formatMoney(150000) = %q, want 'Rp 150.000'", got)
	}

	// formatBytes test
	formatBytes := funcs["formatBytes"].(func(int64) string)
	if got := formatBytes(500); got != "500 B" {
		t.Errorf("formatBytes(500) = %q, want '500 B'", got)
	}
	if got := formatBytes(1024 * 1024 * 5); got != "5.0 MB" {
		t.Errorf("formatBytes(5MB) = %q, want '5.0 MB'", got)
	}

	// formatDateShort test
	formatDateShort := funcs["formatDateShort"].(func(time.Time) string)
	testTime := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	if got := formatDateShort(testTime); got != "14 Sep 2026" {
		t.Errorf("formatDateShort() = %q, want '14 Sep 2026'", got)
	}

	// nl2br test
	nl2br := funcs["nl2br"].(func(string) template.HTML)
	if got := nl2br("line1\nline2"); got != "line1<br>line2" {
		t.Errorf("nl2br() = %q, want 'line1<br>line2'", got)
	}

	// add and sub test
	add := funcs["add"].(func(int, int) int)
	if got := add(2, 3); got != 5 {
		t.Errorf("add(2, 3) = %d, want 5", got)
	}
	sub := funcs["sub"].(func(int, int) int)
	if got := sub(5, 2); got != 3 {
		t.Errorf("sub(5, 2) = %d, want 3", got)
	}

	// deref tests
	derefInt := funcs["derefInt"].(func(*int) int)
	valInt := 42
	if derefInt(nil) != 0 || derefInt(&valInt) != 42 {
		t.Errorf("derefInt failed")
	}

	derefInt64 := funcs["derefInt64"].(func(*int64) int64)
	valInt64 := int64(100)
	if derefInt64(nil) != 0 || derefInt64(&valInt64) != 100 {
		t.Errorf("derefInt64 failed")
	}

	derefTime := funcs["derefTime"].(func(*time.Time) time.Time)
	now := time.Now()
	if !derefTime(nil).IsZero() || derefTime(&now) != now {
		t.Errorf("derefTime failed")
	}
}

func TestViewRender(t *testing.T) {
	mockFS := fstest.MapFS{
		"layouts/main.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/home.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Hello, {{ .Name }}</h1>{{ end }}`),
		},
	}

	v := view.New(mockFS, false)
	rec := httptest.NewRecorder()
	err := v.Render(rec, "main", "home", map[string]string{"Name": "Sellora"})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	if rec.Code != 200 {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	expectedBody := `<!DOCTYPE html><html><body><h1>Hello, Sellora</h1></body></html>`
	if rec.Body.String() != expectedBody {
		t.Errorf("body = %q, want %q", rec.Body.String(), expectedBody)
	}
}

func TestRealStorefrontHomeTemplate(t *testing.T) {
	// Import and verify actual embedded templates compile and render without error
	v := view.New(templates.FS, false)
	rec := httptest.NewRecorder()
	err := v.Render(rec, "public", "storefront/home", map[string]any{
		"Title":            "Sellora - Test",
		"ActiveNav":        "home",
		"TotalPublished":   3,
		"FeaturedProducts": []any{},
	})
	if err != nil {
		t.Fatalf("failed to render real storefront/home template: %v", err)
	}
	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestRealStorefrontDetailTemplate(t *testing.T) {
	v := view.New(templates.FS, false)

	prod := &domain.Product{
		ID:               1,
		Name:             "Go Masterclass Pro",
		Slug:             "go-masterclass-pro",
		ShortDescription: "Belajar Go dari pemula hingga mahir",
		FullDescription:  "# Selamat Datang\n\nPelajari Go secara mendalam.",
		Price:            150000,
		ThumbnailURL:     "https://images.unsplash.com/photo-cover",
		DemoURL:          "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Images: []domain.ProductImage{
			{ID: 1, ProductID: 1, ImageURL: "https://images.unsplash.com/photo-cover", DisplayOrder: 0},
			{ID: 2, ProductID: 1, ImageURL: "https://images.unsplash.com/photo-2", DisplayOrder: 1},
		},
	}

	rec := httptest.NewRecorder()
	err := v.Render(rec, "public", "storefront/detail", map[string]any{
		"Title":      prod.Name + " - Sellora",
		"ActiveNav":  "catalog",
		"Product":    prod,
		"Files":      []domain.ProductFile{},
		"TotalFiles": 0,
		"TotalSize":  int64(0),
	})
	if err != nil {
		t.Fatalf("failed to render real storefront/detail template: %v", err)
	}
	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	// Verify gallery thumbnail rendered
	if !strings.Contains(body, "gallery-thumbnails-strip") {
		t.Errorf("expected body to contain gallery-thumbnails-strip")
	}
	if !strings.Contains(body, "selectGalleryImage") {
		t.Errorf("expected body to contain selectGalleryImage script")
	}
	// Verify video embed iframe rendered
	if !strings.Contains(body, "youtube-nocookie.com/embed/dQw4w9WgXcQ") {
		t.Errorf("expected body to contain youtube embed URL")
	}
}

func TestRealAdminProductImagesTemplate(t *testing.T) {
	v := view.New(templates.FS, false)

	prod := &domain.Product{
		ID:   1,
		Name: "Test Product",
	}
	images := []domain.ProductImage{
		{ID: 1, ProductID: 1, ImageURL: "https://example.com/test.jpg", DisplayOrder: 0},
	}

	rec := httptest.NewRecorder()
	err := v.Render(rec, "admin", "admin/products/images", map[string]any{
		"Title":     "Product Images",
		"ActiveNav": "products",
		"Product":   prod,
		"Images":    images,
		"Error":     "",
	})
	if err != nil {
		t.Fatalf("failed to render real admin/products/images template: %v", err)
	}
	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestRealAdminCouponsTemplates(t *testing.T) {
	v := view.New(templates.FS, false)

	limit := 100
	capAmount := int64(25000)
	now := time.Now().UTC()
	future := now.Add(48 * time.Hour)

	couponList := []domain.Coupon{
		{
			ID:                1,
			Code:              "DISKON20",
			DiscountType:      domain.DiscountTypePercentage,
			DiscountValue:     20,
			MinPurchaseAmount: 50000,
			MaxDiscountAmount: &capAmount,
			UsageLimit:        &limit,
			UsedCount:         12,
			IsActive:          true,
			StartsAt:          &now,
			ExpiresAt:         &future,
		},
	}

	// 1. Render index
	recIndex := httptest.NewRecorder()
	err := v.Render(recIndex, "admin", "admin/coupons/index", map[string]any{
		"Title":     "Coupons & Discounts",
		"ActiveNav": "coupons",
		"Coupons":   couponList,
		"Total":     1,
		"Page":      1,
	})
	if err != nil {
		t.Fatalf("failed to render real admin/coupons/index template: %v", err)
	}
	if recIndex.Code != 200 {
		t.Fatalf("expected status 200, got %d", recIndex.Code)
	}
	if !strings.Contains(recIndex.Body.String(), "DISKON20") {
		t.Errorf("expected rendered index to contain DISKON20")
	}

	// 2. Render form (edit mode)
	recForm := httptest.NewRecorder()
	err = v.Render(recForm, "admin", "admin/coupons/form", map[string]any{
		"Title":     "Edit Coupon - DISKON20",
		"ActiveNav": "coupons",
		"IsEdit":    true,
		"Coupon":    &couponList[0],
		"Error":     "",
	})
	if err != nil {
		t.Fatalf("failed to render real admin/coupons/form template: %v", err)
	}
	if recForm.Code != 200 {
		t.Fatalf("expected status 200, got %d", recForm.Code)
	}
	if !strings.Contains(recForm.Body.String(), "DISKON20") {
		t.Errorf("expected rendered form to contain DISKON20")
	}
}

func TestRealStorefrontCheckoutTemplates(t *testing.T) {
	v := view.New(templates.FS, false)

	prod := &domain.Product{
		ID:    1,
		Name:  "Belajar Go Modern",
		Price: 100000,
		Slug:  "belajar-go-modern",
	}

	c := &domain.Coupon{
		ID:            5,
		Code:          "HEMAT20",
		DiscountType:  domain.DiscountTypePercentage,
		DiscountValue: 20,
	}

	// 1. Checkout page with coupon
	recCheckout := httptest.NewRecorder()
	err := v.Render(recCheckout, "public", "storefront/checkout", map[string]any{
		"Title":           "Checkout - " + prod.Name,
		"User":            &domain.User{Name: "Budi", Email: "budi@example.com"},
		"Product":         prod,
		"PaymentProvider": "MIDTRANS",
		"ClientKey":       "test-client-key",
		"SnapScriptURL":   "https://app.sandbox.midtrans.com/snap/snap.js",
		"Coupon":          c,
		"CouponCode":      "HEMAT20",
		"DiscountAmount":  int64(20000),
		"TotalAmount":     int64(80000),
		"CouponSuccess":   "Kupon HEMAT20 berhasil diterapkan!",
	})
	if err != nil {
		t.Fatalf("failed to render real storefront/checkout template: %v", err)
	}
	if recCheckout.Code != 200 {
		t.Fatalf("expected status 200, got %d", recCheckout.Code)
	}
	body := recCheckout.Body.String()
	if !strings.Contains(body, "HEMAT20") {
		t.Errorf("expected rendered checkout to contain HEMAT20")
	}
	if !strings.Contains(body, "checkout-summary") {
		t.Errorf("expected rendered checkout to contain checkout-summary ID")
	}

	// 2. Order success page with coupon
	order := &domain.Order{
		Reference:      "ORD-2026-TEST",
		Status:         domain.StatusPaid,
		TotalAmount:    80000,
		DiscountAmount: 20000,
		Coupon:         c,
		PaymentProvider: "midtrans",
		Items: []domain.OrderItem{
			{ProductID: 1, ProductName: prod.Name, Price: 100000},
		},
	}
	recSuccess := httptest.NewRecorder()
	err = v.Render(recSuccess, "public", "storefront/order_success", map[string]any{
		"Title": "Pembayaran Berhasil",
		"Order": order,
	})
	if err != nil {
		t.Fatalf("failed to render real storefront/order_success template: %v", err)
	}
	if recSuccess.Code != 200 {
		t.Fatalf("expected status 200, got %d", recSuccess.Code)
	}
	if !strings.Contains(recSuccess.Body.String(), "Potongan Kupon") {
		t.Errorf("expected rendered success to contain Potongan Kupon")
	}
}




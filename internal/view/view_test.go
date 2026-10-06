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



package product_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/product"
	"github.com/yusqohid/go-htmx-ecommerce/internal/storage"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

func setupStorefrontTest(t *testing.T) (*product.Service, *product.StorefrontHandler) {
	t.Helper()
	productRepo := NewMockProductRepository()
	fileRepo := NewMockProductFileRepository()
	storageMgr, _ := storage.New(t.TempDir())
	service := product.NewService(productRepo, fileRepo, storageMgr)

	mockFS := fstest.MapFS{
		"layouts/public.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><head><title>{{ .Title }}</title></head><body>{{ template "content" . }}</body></html>`),
		},
		"pages/storefront/home.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Home</h1><div>Total: {{ .TotalPublished }}</div>{{ end }}`),
		},
		"pages/storefront/catalog.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Catalog</h1><div>Found: {{ .Total }}</div>{{ end }}`),
		},
		"pages/storefront/detail.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Detail: {{ .Product.Name }}</h1>{{ end }}`),
		},
		"pages/storefront/404.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>404 Not Found</h1>{{ end }}`),
		},
	}

	renderer := view.New(mockFS, false)
	handler := product.NewStorefrontHandler(service, renderer)

	return service, handler
}

func TestStorefrontHandler_Home(t *testing.T) {
	service, handler := setupStorefrontTest(t)
	ctx := context.Background()

	// Seed one published product
	_, err := service.CreateProduct(ctx, product.CreateProductInput{
		Name:             "Go Microservices Kit",
		Slug:             "go-microservices-kit",
		ShortDescription: "Production-ready Go kit",
		Price:            150000,
		Status:           domain.StatusPublished,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.Home(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rec.Code)
	}
}

func TestStorefrontHandler_Catalog(t *testing.T) {
	service, handler := setupStorefrontTest(t)
	ctx := context.Background()

	_, _ = service.CreateProduct(ctx, product.CreateProductInput{
		Name:             "Go Backend Course",
		Slug:             "go-backend-course",
		ShortDescription: "Deep dive course",
		Price:            200000,
		Status:           domain.StatusPublished,
	})

	// 1. Regular catalog listing
	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	rec := httptest.NewRecorder()
	handler.Catalog(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}

	// 2. Search query matching
	reqSearch := httptest.NewRequest(http.MethodGet, "/products?q=Backend", nil)
	recSearch := httptest.NewRecorder()
	handler.Catalog(recSearch, reqSearch)
	if recSearch.Code != http.StatusOK {
		t.Errorf("expected 200 OK for search, got %d", recSearch.Code)
	}
}

func TestStorefrontHandler_ProductDetail(t *testing.T) {
	service, handler := setupStorefrontTest(t)
	ctx := context.Background()

	publishedProd, _ := service.CreateProduct(ctx, product.CreateProductInput{
		Name:             "Awesome Theme",
		Slug:             "awesome-theme",
		ShortDescription: "Clean theme",
		Price:            99000,
		Status:           domain.StatusPublished,
	})

	draftProd, _ := service.CreateProduct(ctx, product.CreateProductInput{
		Name:             "Unreleased Theme",
		Slug:             "unreleased-theme",
		ShortDescription: "Draft theme",
		Price:            89000,
		Status:           domain.StatusDraft,
	})

	// Helper to route request with Chi URLParam
	executeWithSlug := func(slug string) *httptest.ResponseRecorder {
		r := chi.NewRouter()
		r.Get("/products/{slug}", handler.ProductDetail)
		req := httptest.NewRequest(http.MethodGet, "/products/"+slug, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 1. Published product should return 200
	recPub := executeWithSlug(publishedProd.Slug)
	if recPub.Code != http.StatusOK {
		t.Errorf("expected 200 OK for published product, got %d", recPub.Code)
	}

	// 2. Draft product should return 404 (hidden from public)
	recDraft := executeWithSlug(draftProd.Slug)
	if recDraft.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for draft product, got %d", recDraft.Code)
	}

	// 3. Non-existent slug should return 404
	recNotFound := executeWithSlug("non-existent-product-slug")
	if recNotFound.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for missing product, got %d", recNotFound.Code)
	}
}

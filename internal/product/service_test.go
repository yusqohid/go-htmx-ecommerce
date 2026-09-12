package product_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/textproto"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/product"
	"github.com/yusqohid/go-htmx-ecommerce/internal/storage"
)

// MockProductRepository is an in-memory repository for unit testing.
type MockProductRepository struct {
	products map[int64]*domain.Product
	bySlug   map[string]*domain.Product
	nextID   int64
}

func NewMockProductRepository() *MockProductRepository {
	return &MockProductRepository{
		products: make(map[int64]*domain.Product),
		bySlug:   make(map[string]*domain.Product),
		nextID:   1,
	}
}

func (m *MockProductRepository) Create(ctx context.Context, p *domain.Product) error {
	if _, exists := m.bySlug[p.Slug]; exists {
		return domain.ErrConflict
	}
	p.ID = m.nextID
	m.nextID++
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	m.products[p.ID] = p
	m.bySlug[p.Slug] = p
	return nil
}

func (m *MockProductRepository) FindByID(ctx context.Context, id int64) (*domain.Product, error) {
	p, ok := m.products[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (m *MockProductRepository) FindBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	p, ok := m.bySlug[slug]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (m *MockProductRepository) ListPublished(ctx context.Context, search string, limit, offset int) ([]domain.Product, int, error) {
	var res []domain.Product
	for _, p := range m.products {
		if p.Status == domain.StatusPublished {
			res = append(res, *p)
		}
	}
	return res, len(res), nil
}

func (m *MockProductRepository) ListAll(ctx context.Context, search string, status string, limit, offset int) ([]domain.Product, int, error) {
	var res []domain.Product
	for _, p := range m.products {
		if status == "" || status == "all" || string(p.Status) == status {
			res = append(res, *p)
		}
	}
	return res, len(res), nil
}

func (m *MockProductRepository) Update(ctx context.Context, p *domain.Product) error {
	if _, ok := m.products[p.ID]; !ok {
		return domain.ErrNotFound
	}
	p.UpdatedAt = time.Now()
	m.products[p.ID] = p
	m.bySlug[p.Slug] = p
	return nil
}

func (m *MockProductRepository) Delete(ctx context.Context, id int64) error {
	p, ok := m.products[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.bySlug, p.Slug)
	delete(m.products, id)
	return nil
}

// MockProductFileRepository is an in-memory repository for unit testing.
type MockProductFileRepository struct {
	files  map[int64]*domain.ProductFile
	nextID int64
}

func NewMockProductFileRepository() *MockProductFileRepository {
	return &MockProductFileRepository{
		files:  make(map[int64]*domain.ProductFile),
		nextID: 1,
	}
}

func (m *MockProductFileRepository) Create(ctx context.Context, f *domain.ProductFile) error {
	f.ID = m.nextID
	m.nextID++
	f.CreatedAt = time.Now()
	m.files[f.ID] = f
	return nil
}

func (m *MockProductFileRepository) FindByID(ctx context.Context, id int64) (*domain.ProductFile, error) {
	f, ok := m.files[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return f, nil
}

func (m *MockProductFileRepository) FindByProductID(ctx context.Context, productID int64) ([]domain.ProductFile, error) {
	var res []domain.ProductFile
	for _, f := range m.files {
		if f.ProductID == productID {
			res = append(res, *f)
		}
	}
	return res, nil
}

func (m *MockProductFileRepository) Delete(ctx context.Context, id int64) error {
	if _, ok := m.files[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.files, id)
	return nil
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Go HTMX E-Commerce Boilerplate!", "go-htmx-e-commerce-boilerplate"},
		{"Special @#$ Characters & Symbols", "special-characters-symbols"},
		{"   Multiple    Spaces   ", "multiple-spaces"},
		{"Already-Clean-Slug", "already-clean-slug"},
	}

	for _, tt := range tests {
		actual := product.Slugify(tt.input)
		if actual != tt.expected {
			t.Errorf("Slugify(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestCreateProduct(t *testing.T) {
	productRepo := NewMockProductRepository()
	fileRepo := NewMockProductFileRepository()
	tempDir := t.TempDir()
	storageMgr, _ := storage.New(tempDir)

	service := product.NewService(productRepo, fileRepo, storageMgr)
	ctx := context.Background()

	// 1. Success case
	prod, err := service.CreateProduct(ctx, product.CreateProductInput{
		Name:  "Test Software Kit",
		Price: 150000,
	})
	if err != nil {
		t.Fatalf("expected product creation to succeed, got %v", err)
	}

	if prod.ID == 0 || prod.Slug != "test-software-kit" || prod.Status != domain.StatusDraft {
		t.Errorf("unexpected product properties: %+v", prod)
	}

	// 2. Empty name should fail
	_, err = service.CreateProduct(ctx, product.CreateProductInput{
		Name:  "",
		Price: 10000,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty name, got %v", err)
	}

	// 3. Negative price should fail
	_, err = service.CreateProduct(ctx, product.CreateProductInput{
		Name:  "Invalid Price Product",
		Price: -500,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for negative price, got %v", err)
	}
}

func TestTogglePublish(t *testing.T) {
	productRepo := NewMockProductRepository()
	fileRepo := NewMockProductFileRepository()
	tempDir := t.TempDir()
	storageMgr, _ := storage.New(tempDir)

	service := product.NewService(productRepo, fileRepo, storageMgr)
	ctx := context.Background()

	prod, _ := service.CreateProduct(ctx, product.CreateProductInput{
		Name:  "Draft Kit",
		Price: 50000,
	})

	// Toggle 1: Draft -> Published
	updated, err := service.TogglePublish(ctx, prod.ID)
	if err != nil {
		t.Fatalf("failed to toggle publish: %v", err)
	}
	if updated.Status != domain.StatusPublished {
		t.Errorf("expected status 'published', got %s", updated.Status)
	}

	// Toggle 2: Published -> Draft
	updated2, err := service.TogglePublish(ctx, prod.ID)
	if err != nil {
		t.Fatalf("failed to toggle draft: %v", err)
	}
	if updated2.Status != domain.StatusDraft {
		t.Errorf("expected status 'draft', got %s", updated2.Status)
	}
}

func TestUploadAndDeleteFile(t *testing.T) {
	productRepo := NewMockProductRepository()
	fileRepo := NewMockProductFileRepository()
	tempDir := t.TempDir()
	storageMgr, _ := storage.New(tempDir)

	service := product.NewService(productRepo, fileRepo, storageMgr)
	ctx := context.Background()

	prod, _ := service.CreateProduct(ctx, product.CreateProductInput{
		Name:  "Digital Asset",
		Price: 100000,
	})

	// Create a mock multipart.FileHeader
	content := []byte("Hello, this is digital downloadable source code content!")
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="asset.zip"`)
	h.Set("Content-Type", "application/zip")
	part, _ := w.CreatePart(h)
	_, _ = io.Copy(part, bytes.NewReader(content))
	_ = w.Close()

	reader := multipart.NewReader(&b, w.Boundary())
	form, err := reader.ReadForm(10 * 1024)
	if err != nil {
		t.Fatalf("failed to read test form: %v", err)
	}

	fileHeader := form.File["file"][0]

	// 1. Upload File
	productFile, err := service.UploadFile(ctx, prod.ID, fileHeader, "v1.0.0")
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	if productFile.OriginalName != "asset.zip" || productFile.Version != "v1.0.0" {
		t.Errorf("unexpected product file values: %+v", productFile)
	}

	if productFile.Checksum == "" || productFile.FileSize != int64(len(content)) {
		t.Errorf("invalid checksum or size: %+v", productFile)
	}

	// Verify file exists on disk
	f, err := storageMgr.Open(productFile.StoragePath)
	if err != nil {
		t.Fatalf("expected stored file to open: %v", err)
	}
	f.Close()

	// 2. Delete File
	err = service.DeleteFile(ctx, productFile.ID)
	if err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	// Record should be deleted
	_, err = fileRepo.FindByID(ctx, productFile.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for deleted file, got %v", err)
	}
	_ = filepath.Base
}

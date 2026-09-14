package product

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/storage"
)

const MaxFileSize = 150 * 1024 * 1024 // 150 MB max digital product upload

// CreateProductInput holds data required to create a new product.
type CreateProductInput struct {
	Name             string
	Slug             string
	ShortDescription string
	FullDescription  string
	Price            int64
	ThumbnailURL     string
	Status           domain.ProductStatus
}

// UpdateProductInput holds data required to update an existing product.
type UpdateProductInput struct {
	Name             string
	Slug             string
	ShortDescription string
	FullDescription  string
	Price            int64
	ThumbnailURL     string
	Status           domain.ProductStatus
}

// Service manages product catalog business rules.
type Service struct {
	productRepo domain.ProductRepository
	fileRepo    domain.ProductFileRepository
	storage     *storage.Storage
}

// NewService creates a new product Service instance.
func NewService(
	productRepo domain.ProductRepository,
	fileRepo domain.ProductFileRepository,
	storage *storage.Storage,
) *Service {
	return &Service{
		productRepo: productRepo,
		fileRepo:    fileRepo,
		storage:     storage,
	}
}

// CreateProduct validates and stores a new digital product.
func (s *Service) CreateProduct(ctx context.Context, input CreateProductInput) (*domain.Product, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: product name is required", domain.ErrInvalidInput)
	}

	if input.Price < 0 {
		return nil, fmt.Errorf("%w: price cannot be negative", domain.ErrInvalidInput)
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = Slugify(name)
	} else {
		slug = Slugify(slug)
	}

	status := input.Status
	if status == "" {
		status = domain.StatusDraft
	}

	product := &domain.Product{
		Name:             name,
		Slug:             slug,
		ShortDescription: strings.TrimSpace(input.ShortDescription),
		FullDescription:  strings.TrimSpace(input.FullDescription),
		Price:            input.Price,
		ThumbnailURL:     strings.TrimSpace(input.ThumbnailURL),
		Status:           status,
	}

	// Attempt insertion, append timestamp suffix if slug conflict occurs
	err := s.productRepo.Create(ctx, product)
	if errors.Is(err, domain.ErrConflict) {
		product.Slug = fmt.Sprintf("%s-%d", slug, time.Now().Unix())
		err = s.productRepo.Create(ctx, product)
	}
	if err != nil {
		return nil, err
	}

	return product, nil
}

// UpdateProduct updates product metadata.
func (s *Service) UpdateProduct(ctx context.Context, id int64, input UpdateProductInput) (*domain.Product, error) {
	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: product name cannot be empty", domain.ErrInvalidInput)
	}

	if input.Price < 0 {
		return nil, fmt.Errorf("%w: price cannot be negative", domain.ErrInvalidInput)
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = Slugify(name)
	} else {
		slug = Slugify(slug)
	}

	product.Name = name
	product.Slug = slug
	product.ShortDescription = strings.TrimSpace(input.ShortDescription)
	product.FullDescription = strings.TrimSpace(input.FullDescription)
	product.Price = input.Price
	product.ThumbnailURL = strings.TrimSpace(input.ThumbnailURL)
	if input.Status != "" {
		product.Status = input.Status
	}

	if err := s.productRepo.Update(ctx, product); err != nil {
		return nil, err
	}

	return product, nil
}

// TogglePublish switches a product status between draft and published.
func (s *Service) TogglePublish(ctx context.Context, id int64) (*domain.Product, error) {
	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if product.Status == domain.StatusPublished {
		product.Status = domain.StatusDraft
	} else {
		product.Status = domain.StatusPublished
	}

	if err := s.productRepo.Update(ctx, product); err != nil {
		return nil, err
	}

	return product, nil
}

// UploadFile validates and stores an uploaded file for a product.
func (s *Service) UploadFile(ctx context.Context, productID int64, fileHeader *multipart.FileHeader, version string) (*domain.ProductFile, error) {
	// Verify product exists
	_, err := s.productRepo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if fileHeader.Size > MaxFileSize {
		return nil, fmt.Errorf("%w: file size exceeds maximum limit of 150MB", domain.ErrInvalidInput)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	// Detect MIME type from first 512 bytes
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	mimeType := http.DetectContentType(buffer[:n])
	_, _ = file.Seek(0, io.SeekStart)

	// Save file to secure storage
	storageKey, size, checksum, err := s.storage.Save(file, fileHeader.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	if version == "" {
		version = "1.0.0"
	}

	productFile := &domain.ProductFile{
		ProductID:    productID,
		OriginalName: fileHeader.Filename,
		StoragePath:  storageKey,
		FileSize:     size,
		MIMEType:     mimeType,
		Version:      strings.TrimSpace(version),
		Checksum:     checksum,
	}

	if err := s.fileRepo.Create(ctx, productFile); err != nil {
		// Clean up uploaded file from storage if DB insert fails
		_ = s.storage.Delete(storageKey)
		return nil, fmt.Errorf("failed to record file in database: %w", err)
	}

	return productFile, nil
}

// DeleteFile removes a file from storage and database.
func (s *Service) DeleteFile(ctx context.Context, fileID int64) error {
	file, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return err
	}

	if err := s.storage.Delete(file.StoragePath); err != nil {
		// Log warning but continue with database deletion
	}

	return s.fileRepo.Delete(ctx, fileID)
}

// DeleteProduct deletes a product and all its stored files.
func (s *Service) DeleteProduct(ctx context.Context, id int64) error {
	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete all physical files from disk
	for _, f := range product.Files {
		_ = s.storage.Delete(f.StoragePath)
	}

	return s.productRepo.Delete(ctx, id)
}

// GetProduct retrieves a product by its ID.
func (s *Service) GetProduct(ctx context.Context, id int64) (*domain.Product, error) {
	return s.productRepo.FindByID(ctx, id)
}

// GetProductBySlug retrieves a product by its unique slug.
func (s *Service) GetProductBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	return s.productRepo.FindBySlug(ctx, slug)
}

// GetPublishedProductBySlug retrieves an active product by slug for storefront display,
// returning domain.ErrNotFound if the product is a draft or archived.
func (s *Service) GetPublishedProductBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	product, err := s.productRepo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if product.Status != domain.StatusPublished {
		return nil, domain.ErrNotFound
	}

	return product, nil
}

// ListPublishedProducts retrieves paginated active products for the storefront.
func (s *Service) ListPublishedProducts(ctx context.Context, search string, page, limit int) ([]domain.Product, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 12
	}
	offset := (page - 1) * limit

	return s.productRepo.ListPublished(ctx, search, limit, offset)
}

// ListProducts retrieves paginated products for the admin panel.
func (s *Service) ListProducts(ctx context.Context, search string, status string, page, limit int) ([]domain.Product, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	return s.productRepo.ListAll(ctx, search, status, limit, offset)
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts text into a clean URL-friendly slug.
func Slugify(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	slug := nonAlphanumericRegex.ReplaceAllString(lower, "-")
	return strings.Trim(slug, "-")
}

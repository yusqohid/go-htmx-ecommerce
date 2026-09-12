package domain

import (
	"context"
	"time"
)

// ProductStatus represents the lifecycle state of a product.
type ProductStatus string

const (
	StatusDraft     ProductStatus = "draft"
	StatusPublished ProductStatus = "published"
	StatusArchived  ProductStatus = "archived"
)

// Product represents a digital good in the store.
type Product struct {
	ID               int64         `json:"id"`
	Name             string        `json:"name"`
	Slug             string        `json:"slug"`
	ShortDescription string        `json:"short_description"`
	FullDescription  string        `json:"full_description"`
	Price            int64         `json:"price"` // in smallest currency unit (e.g. IDR)
	ThumbnailURL     string        `json:"thumbnail_url"`
	Status           ProductStatus `json:"status"`
	Files            []ProductFile `json:"files,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// IsPublished checks if the product is active and visible to customers.
func (p *Product) IsPublished() bool {
	return p.Status == StatusPublished
}

// ProductFile represents an uploaded digital asset associated with a product.
type ProductFile struct {
	ID           int64     `json:"id"`
	ProductID    int64     `json:"product_id"`
	OriginalName string    `json:"original_name"`
	StoragePath  string    `json:"-"` // Internal path should never be exposed to public JSON
	FileSize     int64     `json:"file_size"`
	MIMEType     string    `json:"mime_type"`
	Version      string    `json:"version"`
	Checksum     string    `json:"checksum"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProductRepository defines persistence operations for products.
type ProductRepository interface {
	Create(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id int64) (*Product, error)
	FindBySlug(ctx context.Context, slug string) (*Product, error)
	ListPublished(ctx context.Context, search string, limit, offset int) ([]Product, int, error)
	ListAll(ctx context.Context, search string, status string, limit, offset int) ([]Product, int, error)
	Update(ctx context.Context, product *Product) error
	Delete(ctx context.Context, id int64) error
}

// ProductFileRepository defines persistence operations for digital product files.
type ProductFileRepository interface {
	Create(ctx context.Context, file *ProductFile) error
	FindByID(ctx context.Context, id int64) (*ProductFile, error)
	FindByProductID(ctx context.Context, productID int64) ([]ProductFile, error)
	Delete(ctx context.Context, id int64) error
}

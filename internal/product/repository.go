package product

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// PostgresProductRepository implements domain.ProductRepository using PostgreSQL.
type PostgresProductRepository struct {
	db *sql.DB
}

// NewProductRepository creates a new PostgresProductRepository.
func NewProductRepository(db *sql.DB) *PostgresProductRepository {
	return &PostgresProductRepository{db: db}
}

// Create inserts a new product into the database.
func (r *PostgresProductRepository) Create(ctx context.Context, p *domain.Product) error {
	query := `
		INSERT INTO products (name, slug, short_description, full_description, price, thumbnail_url, demo_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at;
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query,
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.Slug),
		p.ShortDescription,
		p.FullDescription,
		p.Price,
		p.ThumbnailURL,
		strings.TrimSpace(p.DemoURL),
		p.Status,
		now,
		now,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "idx_products_slug") || strings.Contains(err.Error(), "products_slug_key") {
			return fmt.Errorf("%w: product with slug '%s' already exists", domain.ErrConflict, p.Slug)
		}
		return fmt.Errorf("failed to create product: %w", err)
	}

	return nil
}

// FindByID retrieves a product by its ID, including its associated digital files and gallery images.
func (r *PostgresProductRepository) FindByID(ctx context.Context, id int64) (*domain.Product, error) {
	query := `
		SELECT id, name, slug, short_description, full_description, price, thumbnail_url, demo_url, status, created_at, updated_at
		FROM products
		WHERE id = $1;
	`
	p := &domain.Product{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.ShortDescription,
		&p.FullDescription,
		&p.Price,
		&p.ThumbnailURL,
		&p.DemoURL,
		&p.Status,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find product by id: %w", err)
	}

	files, err := r.findFilesByProductID(ctx, p.ID)
	if err == nil {
		p.Files = files
	}

	images, err := r.findImagesByProductID(ctx, p.ID)
	if err == nil {
		p.Images = images
	}

	return p, nil
}

// FindBySlug retrieves a product by its unique slug.
func (r *PostgresProductRepository) FindBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	query := `
		SELECT id, name, slug, short_description, full_description, price, thumbnail_url, demo_url, status, created_at, updated_at
		FROM products
		WHERE slug = $1;
	`
	p := &domain.Product{}
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(slug)).Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.ShortDescription,
		&p.FullDescription,
		&p.Price,
		&p.ThumbnailURL,
		&p.DemoURL,
		&p.Status,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find product by slug: %w", err)
	}

	files, err := r.findFilesByProductID(ctx, p.ID)
	if err == nil {
		p.Files = files
	}

	images, err := r.findImagesByProductID(ctx, p.ID)
	if err == nil {
		p.Images = images
	}

	return p, nil
}

// ListPublished retrieves published products for storefront catalog.
func (r *PostgresProductRepository) ListPublished(ctx context.Context, search string, limit, offset int) ([]domain.Product, int, error) {
	searchPattern := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"

	countQuery := `
		SELECT COUNT(*)
		FROM products
		WHERE status = 'published' AND (LOWER(name) LIKE $1 OR LOWER(short_description) LIKE $1);
	`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, searchPattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count published products: %w", err)
	}

	query := `
		SELECT id, name, slug, short_description, full_description, price, thumbnail_url, demo_url, status, created_at, updated_at
		FROM products
		WHERE status = 'published' AND (LOWER(name) LIKE $1 OR LOWER(short_description) LIKE $1)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3;
	`
	rows, err := r.db.QueryContext(ctx, query, searchPattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query published products: %w", err)
	}
	defer rows.Close()

	var products []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Slug,
			&p.ShortDescription,
			&p.FullDescription,
			&p.Price,
			&p.ThumbnailURL,
			&p.DemoURL,
			&p.Status,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}

	return products, total, nil
}

// ListAll retrieves products for admin management with optional status filter.
func (r *PostgresProductRepository) ListAll(ctx context.Context, search string, status string, limit, offset int) ([]domain.Product, int, error) {
	searchPattern := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"

	whereClauses := []string{"(LOWER(name) LIKE $1 OR LOWER(slug) LIKE $1)"}
	args := []any{searchPattern}
	argIdx := 2

	if status != "" && status != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM products WHERE %s;", whereSQL)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, name, slug, short_description, full_description, price, thumbnail_url, demo_url, status, created_at, updated_at
		FROM products
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;
	`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	var products []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Slug,
			&p.ShortDescription,
			&p.FullDescription,
			&p.Price,
			&p.ThumbnailURL,
			&p.DemoURL,
			&p.Status,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}

	// Fetch file count and image count for each product
	for i := range products {
		files, _ := r.findFilesByProductID(ctx, products[i].ID)
		products[i].Files = files
		images, _ := r.findImagesByProductID(ctx, products[i].ID)
		products[i].Images = images
	}

	return products, total, nil
}

// Update modifies an existing product.
func (r *PostgresProductRepository) Update(ctx context.Context, p *domain.Product) error {
	query := `
		UPDATE products
		SET name = $1, slug = $2, short_description = $3, full_description = $4, price = $5, thumbnail_url = $6, demo_url = $7, status = $8, updated_at = $9
		WHERE id = $10;
	`
	p.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query,
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.Slug),
		p.ShortDescription,
		p.FullDescription,
		p.Price,
		p.ThumbnailURL,
		strings.TrimSpace(p.DemoURL),
		p.Status,
		p.UpdatedAt,
		p.ID,
	)

	if err != nil {
		if strings.Contains(err.Error(), "idx_products_slug") || strings.Contains(err.Error(), "products_slug_key") {
			return fmt.Errorf("%w: product with slug '%s' already exists", domain.ErrConflict, p.Slug)
		}
		return fmt.Errorf("failed to update product: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Delete removes a product by its ID.
func (r *PostgresProductRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM products WHERE id = $1;`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CountStats returns count of total, published, and draft products.
func (r *PostgresProductRepository) CountStats(ctx context.Context) (total int, published int, drafts int, err error) {
	query := `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'published'),
			COUNT(*) FILTER (WHERE status = 'draft')
		FROM products;
	`
	err = r.db.QueryRowContext(ctx, query).Scan(&total, &published, &drafts)
	return
}

func (r *PostgresProductRepository) findFilesByProductID(ctx context.Context, productID int64) ([]domain.ProductFile, error) {
	query := `
		SELECT id, product_id, original_name, storage_path, file_size, mime_type, version, checksum, created_at
		FROM product_files
		WHERE product_id = $1
		ORDER BY created_at DESC;
	`
	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []domain.ProductFile
	for rows.Next() {
		var f domain.ProductFile
		if err := rows.Scan(
			&f.ID,
			&f.ProductID,
			&f.OriginalName,
			&f.StoragePath,
			&f.FileSize,
			&f.MIMEType,
			&f.Version,
			&f.Checksum,
			&f.CreatedAt,
		); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (r *PostgresProductRepository) findImagesByProductID(ctx context.Context, productID int64) ([]domain.ProductImage, error) {
	query := `
		SELECT id, product_id, image_url, display_order, created_at
		FROM product_images
		WHERE product_id = $1
		ORDER BY display_order ASC, created_at ASC;
	`
	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []domain.ProductImage
	for rows.Next() {
		var img domain.ProductImage
		if err := rows.Scan(
			&img.ID,
			&img.ProductID,
			&img.ImageURL,
			&img.DisplayOrder,
			&img.CreatedAt,
		); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

// PostgresProductFileRepository implements domain.ProductFileRepository using PostgreSQL.
type PostgresProductFileRepository struct {
	db *sql.DB
}

// NewProductFileRepository creates a new PostgresProductFileRepository.
func NewProductFileRepository(db *sql.DB) *PostgresProductFileRepository {
	return &PostgresProductFileRepository{db: db}
}

// Create stores metadata for an uploaded digital product file.
func (r *PostgresProductFileRepository) Create(ctx context.Context, f *domain.ProductFile) error {
	query := `
		INSERT INTO product_files (product_id, original_name, storage_path, file_size, mime_type, version, checksum, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at;
	`
	f.CreatedAt = time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query,
		f.ProductID,
		strings.TrimSpace(f.OriginalName),
		f.StoragePath,
		f.FileSize,
		f.MIMEType,
		strings.TrimSpace(f.Version),
		f.Checksum,
		f.CreatedAt,
	).Scan(&f.ID, &f.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert product file: %w", err)
	}
	return nil
}

// FindByID retrieves a product file by its ID.
func (r *PostgresProductFileRepository) FindByID(ctx context.Context, id int64) (*domain.ProductFile, error) {
	query := `
		SELECT id, product_id, original_name, storage_path, file_size, mime_type, version, checksum, created_at
		FROM product_files
		WHERE id = $1;
	`
	f := &domain.ProductFile{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&f.ID,
		&f.ProductID,
		&f.OriginalName,
		&f.StoragePath,
		&f.FileSize,
		&f.MIMEType,
		&f.Version,
		&f.Checksum,
		&f.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find product file by id: %w", err)
	}

	return f, nil
}

// FindByProductID retrieves all files associated with a product.
func (r *PostgresProductFileRepository) FindByProductID(ctx context.Context, productID int64) ([]domain.ProductFile, error) {
	query := `
		SELECT id, product_id, original_name, storage_path, file_size, mime_type, version, checksum, created_at
		FROM product_files
		WHERE product_id = $1
		ORDER BY created_at DESC;
	`
	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to list product files: %w", err)
	}
	defer rows.Close()

	var files []domain.ProductFile
	for rows.Next() {
		var f domain.ProductFile
		if err := rows.Scan(
			&f.ID,
			&f.ProductID,
			&f.OriginalName,
			&f.StoragePath,
			&f.FileSize,
			&f.MIMEType,
			&f.Version,
			&f.Checksum,
			&f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan product file: %w", err)
		}
		files = append(files, f)
	}

	return files, nil
}

// Delete removes a product file by its ID.
func (r *PostgresProductFileRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM product_files WHERE id = $1;`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product file: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// PostgresProductImageRepository implements domain.ProductImageRepository using PostgreSQL.
type PostgresProductImageRepository struct {
	db *sql.DB
}

// NewProductImageRepository creates a new PostgresProductImageRepository.
func NewProductImageRepository(db *sql.DB) *PostgresProductImageRepository {
	return &PostgresProductImageRepository{db: db}
}

// Create stores metadata for a product gallery image.
func (r *PostgresProductImageRepository) Create(ctx context.Context, img *domain.ProductImage) error {
	query := `
		INSERT INTO product_images (product_id, image_url, display_order, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at;
	`
	img.CreatedAt = time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query,
		img.ProductID,
		strings.TrimSpace(img.ImageURL),
		img.DisplayOrder,
		img.CreatedAt,
	).Scan(&img.ID, &img.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert product image: %w", err)
	}
	return nil
}

// FindByID retrieves a product image by its ID.
func (r *PostgresProductImageRepository) FindByID(ctx context.Context, id int64) (*domain.ProductImage, error) {
	query := `
		SELECT id, product_id, image_url, display_order, created_at
		FROM product_images
		WHERE id = $1;
	`
	img := &domain.ProductImage{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&img.ID,
		&img.ProductID,
		&img.ImageURL,
		&img.DisplayOrder,
		&img.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find product image by id: %w", err)
	}

	return img, nil
}

// FindByProductID retrieves all images associated with a product ordered by display_order.
func (r *PostgresProductImageRepository) FindByProductID(ctx context.Context, productID int64) ([]domain.ProductImage, error) {
	query := `
		SELECT id, product_id, image_url, display_order, created_at
		FROM product_images
		WHERE product_id = $1
		ORDER BY display_order ASC, created_at ASC;
	`
	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to list product images: %w", err)
	}
	defer rows.Close()

	var images []domain.ProductImage
	for rows.Next() {
		var img domain.ProductImage
		if err := rows.Scan(
			&img.ID,
			&img.ProductID,
			&img.ImageURL,
			&img.DisplayOrder,
			&img.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan product image: %w", err)
		}
		images = append(images, img)
	}

	return images, nil
}

// Delete removes a product image by its ID.
func (r *PostgresProductImageRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM product_images WHERE id = $1;`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product image: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

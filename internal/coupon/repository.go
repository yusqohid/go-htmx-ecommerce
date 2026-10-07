package coupon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// PostgresCouponRepository implements domain.CouponRepository using PostgreSQL.
type PostgresCouponRepository struct {
	db *sql.DB
}

// NewCouponRepository creates a new PostgresCouponRepository.
func NewCouponRepository(db *sql.DB) *PostgresCouponRepository {
	return &PostgresCouponRepository{db: db}
}

// Create inserts a new coupon into the database.
func (r *PostgresCouponRepository) Create(ctx context.Context, c *domain.Coupon) error {
	now := time.Now().UTC()
	c.Code = domain.NormalizeCouponCode(c.Code)

	query := `
		INSERT INTO coupons (
			code, discount_type, discount_value, min_purchase_amount,
			max_discount_amount, usage_limit, used_count, is_active,
			starts_at, expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at;
	`

	err := r.db.QueryRowContext(ctx, query,
		c.Code,
		string(c.DiscountType),
		c.DiscountValue,
		c.MinPurchaseAmount,
		c.MaxDiscountAmount,
		c.UsageLimit,
		c.UsedCount,
		c.IsActive,
		c.StartsAt,
		c.ExpiresAt,
		now,
		now,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.ErrConflict
		}
		return fmt.Errorf("failed to insert coupon: %w", err)
	}

	return nil
}

// FindByID retrieves a coupon by its primary key ID.
func (r *PostgresCouponRepository) FindByID(ctx context.Context, id int64) (*domain.Coupon, error) {
	query := `
		SELECT id, code, discount_type, discount_value, min_purchase_amount,
		       max_discount_amount, usage_limit, used_count, is_active,
		       starts_at, expires_at, created_at, updated_at
		FROM coupons
		WHERE id = $1;
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanCoupon(row)
}

// FindByCode retrieves a coupon by its unique code (case-insensitive normalized).
func (r *PostgresCouponRepository) FindByCode(ctx context.Context, code string) (*domain.Coupon, error) {
	normalized := domain.NormalizeCouponCode(code)
	query := `
		SELECT id, code, discount_type, discount_value, min_purchase_amount,
		       max_discount_amount, usage_limit, used_count, is_active,
		       starts_at, expires_at, created_at, updated_at
		FROM coupons
		WHERE code = $1;
	`
	row := r.db.QueryRowContext(ctx, query, normalized)
	return scanCoupon(row)
}

// ListAll retrieves paginated coupons ordered by creation date descending.
func (r *PostgresCouponRepository) ListAll(ctx context.Context, limit, offset int) ([]domain.Coupon, int, error) {
	countQuery := `SELECT COUNT(*) FROM coupons;`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count coupons: %w", err)
	}

	query := `
		SELECT id, code, discount_type, discount_value, min_purchase_amount,
		       max_discount_amount, usage_limit, used_count, is_active,
		       starts_at, expires_at, created_at, updated_at
		FROM coupons
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list coupons: %w", err)
	}
	defer rows.Close()

	var coupons []domain.Coupon
	for rows.Next() {
		c, err := scanCouponRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan coupon row: %w", err)
		}
		coupons = append(coupons, *c)
	}

	return coupons, total, nil
}

// Update updates an existing coupon's fields.
func (r *PostgresCouponRepository) Update(ctx context.Context, c *domain.Coupon) error {
	now := time.Now().UTC()
	c.Code = domain.NormalizeCouponCode(c.Code)

	query := `
		UPDATE coupons
		SET code = $1, discount_type = $2, discount_value = $3, min_purchase_amount = $4,
		    max_discount_amount = $5, usage_limit = $6, is_active = $7, starts_at = $8,
		    expires_at = $9, updated_at = $10
		WHERE id = $11;
	`

	res, err := r.db.ExecContext(ctx, query,
		c.Code,
		string(c.DiscountType),
		c.DiscountValue,
		c.MinPurchaseAmount,
		c.MaxDiscountAmount,
		c.UsageLimit,
		c.IsActive,
		c.StartsAt,
		c.ExpiresAt,
		now,
		c.ID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.ErrConflict
		}
		return fmt.Errorf("failed to update coupon: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	c.UpdatedAt = now
	return nil
}

// IncrementUsedCount atomically increments the used_count for a coupon by 1.
func (r *PostgresCouponRepository) IncrementUsedCount(ctx context.Context, id int64) error {
	query := `
		UPDATE coupons
		SET used_count = used_count + 1, updated_at = NOW()
		WHERE id = $1;
	`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to increment coupon used_count: %w", err)
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

// Delete removes a coupon from the database.
func (r *PostgresCouponRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM coupons WHERE id = $1;`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete coupon: %w", err)
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

type scannable interface {
	Scan(dest ...any) error
}

func scanCoupon(s scannable) (*domain.Coupon, error) {
	c := &domain.Coupon{}
	var discType string
	var maxDisc sql.NullInt64
	var usageLimit sql.NullInt64
	var startsAt sql.NullTime
	var expiresAt sql.NullTime

	err := s.Scan(
		&c.ID,
		&c.Code,
		&discType,
		&c.DiscountValue,
		&c.MinPurchaseAmount,
		&maxDisc,
		&usageLimit,
		&c.UsedCount,
		&c.IsActive,
		&startsAt,
		&expiresAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan coupon: %w", err)
	}

	c.DiscountType = domain.DiscountType(discType)
	if maxDisc.Valid {
		val := maxDisc.Int64
		c.MaxDiscountAmount = &val
	}
	if usageLimit.Valid {
		val := int(usageLimit.Int64)
		c.UsageLimit = &val
	}
	if startsAt.Valid {
		val := startsAt.Time
		c.StartsAt = &val
	}
	if expiresAt.Valid {
		val := expiresAt.Time
		c.ExpiresAt = &val
	}

	return c, nil
}

func scanCouponRow(rows *sql.Rows) (*domain.Coupon, error) {
	return scanCoupon(rows)
}

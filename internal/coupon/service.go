package coupon

import (
	"context"
	"fmt"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// Service encapsulates business logic for promotional coupons.
type Service struct {
	repo domain.CouponRepository
}

// NewService creates a new coupon Service.
func NewService(repo domain.CouponRepository) *Service {
	return &Service{repo: repo}
}

// CreateCouponInput specifies the fields needed to create a new coupon.
type CreateCouponInput struct {
	Code              string
	DiscountType      domain.DiscountType
	DiscountValue     int64
	MinPurchaseAmount int64
	MaxDiscountAmount *int64
	UsageLimit        *int
	IsActive          bool
	StartsAt          *time.Time
	ExpiresAt         *time.Time
}

// UpdateCouponInput specifies the fields that can be updated on an existing coupon.
type UpdateCouponInput struct {
	Code              string
	DiscountType      domain.DiscountType
	DiscountValue     int64
	MinPurchaseAmount int64
	MaxDiscountAmount *int64
	UsageLimit        *int
	IsActive          bool
	StartsAt          *time.Time
	ExpiresAt         *time.Time
}

// ValidateAndCalculate checks if a coupon code is applicable to an order subtotal and calculates the discount amount.
func (s *Service) ValidateAndCalculate(ctx context.Context, code string, subtotal int64) (*domain.Coupon, int64, error) {
	normalized := domain.NormalizeCouponCode(code)
	if normalized == "" {
		return nil, 0, domain.ErrCouponInvalidCode
	}

	coupon, err := s.repo.FindByCode(ctx, normalized)
	if err != nil {
		return nil, 0, err
	}

	if err := coupon.Validate(time.Now().UTC(), subtotal); err != nil {
		return nil, 0, err
	}

	discount := coupon.CalculateDiscount(subtotal)
	return coupon, discount, nil
}

// CreateCoupon validates and persists a new coupon.
func (s *Service) CreateCoupon(ctx context.Context, input CreateCouponInput) (*domain.Coupon, error) {
	code := domain.NormalizeCouponCode(input.Code)
	if code == "" {
		return nil, fmt.Errorf("%w: coupon code cannot be empty", domain.ErrInvalidInput)
	}

	if input.DiscountValue <= 0 {
		return nil, fmt.Errorf("%w: discount value must be greater than zero", domain.ErrInvalidInput)
	}

	if input.DiscountType != domain.DiscountTypePercentage && input.DiscountType != domain.DiscountTypeFixed {
		return nil, fmt.Errorf("%w: invalid discount type (must be percentage or fixed)", domain.ErrInvalidInput)
	}

	if input.DiscountType == domain.DiscountTypePercentage && input.DiscountValue > 100 {
		return nil, fmt.Errorf("%w: percentage discount cannot exceed 100%%", domain.ErrInvalidInput)
	}

	if input.StartsAt != nil && input.ExpiresAt != nil && input.ExpiresAt.Before(*input.StartsAt) {
		return nil, fmt.Errorf("%w: expiration date cannot be before start date", domain.ErrInvalidInput)
	}

	coupon := &domain.Coupon{
		Code:              code,
		DiscountType:      input.DiscountType,
		DiscountValue:     input.DiscountValue,
		MinPurchaseAmount: input.MinPurchaseAmount,
		MaxDiscountAmount: input.MaxDiscountAmount,
		UsageLimit:        input.UsageLimit,
		IsActive:          input.IsActive,
		StartsAt:          input.StartsAt,
		ExpiresAt:         input.ExpiresAt,
	}

	if err := s.repo.Create(ctx, coupon); err != nil {
		return nil, err
	}

	return coupon, nil
}

// UpdateCoupon modifies an existing coupon.
func (s *Service) UpdateCoupon(ctx context.Context, id int64, input UpdateCouponInput) (*domain.Coupon, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	code := domain.NormalizeCouponCode(input.Code)
	if code == "" {
		return nil, fmt.Errorf("%w: coupon code cannot be empty", domain.ErrInvalidInput)
	}

	if input.DiscountValue <= 0 {
		return nil, fmt.Errorf("%w: discount value must be greater than zero", domain.ErrInvalidInput)
	}

	if input.DiscountType != domain.DiscountTypePercentage && input.DiscountType != domain.DiscountTypeFixed {
		return nil, fmt.Errorf("%w: invalid discount type (must be percentage or fixed)", domain.ErrInvalidInput)
	}

	if input.DiscountType == domain.DiscountTypePercentage && input.DiscountValue > 100 {
		return nil, fmt.Errorf("%w: percentage discount cannot exceed 100%%", domain.ErrInvalidInput)
	}

	if input.StartsAt != nil && input.ExpiresAt != nil && input.ExpiresAt.Before(*input.StartsAt) {
		return nil, fmt.Errorf("%w: expiration date cannot be before start date", domain.ErrInvalidInput)
	}

	existing.Code = code
	existing.DiscountType = input.DiscountType
	existing.DiscountValue = input.DiscountValue
	existing.MinPurchaseAmount = input.MinPurchaseAmount
	existing.MaxDiscountAmount = input.MaxDiscountAmount
	existing.UsageLimit = input.UsageLimit
	existing.IsActive = input.IsActive
	existing.StartsAt = input.StartsAt
	existing.ExpiresAt = input.ExpiresAt

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// GetCoupon retrieves a coupon by its primary key ID.
func (s *Service) GetCoupon(ctx context.Context, id int64) (*domain.Coupon, error) {
	return s.repo.FindByID(ctx, id)
}

// GetCouponByCode retrieves a coupon by its code.
func (s *Service) GetCouponByCode(ctx context.Context, code string) (*domain.Coupon, error) {
	return s.repo.FindByCode(ctx, domain.NormalizeCouponCode(code))
}

// ListCoupons retrieves paginated coupons for admin display.
func (s *Service) ListCoupons(ctx context.Context, page, limit int) ([]domain.Coupon, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit
	return s.repo.ListAll(ctx, limit, offset)
}

// ToggleCouponStatus flips the is_active status of a coupon.
func (s *Service) ToggleCouponStatus(ctx context.Context, id int64) (*domain.Coupon, error) {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	c.IsActive = !c.IsActive
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// DeleteCoupon removes a coupon.
func (s *Service) DeleteCoupon(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

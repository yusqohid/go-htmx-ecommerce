package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrCouponNotFound is returned when a coupon with the requested code does not exist.
	ErrCouponNotFound = errors.New("coupon not found")
	// ErrCouponInactive is returned when a coupon is marked as inactive.
	ErrCouponInactive = errors.New("coupon is currently inactive")
	// ErrCouponNotStarted is returned when a promotion has a start date in the future.
	ErrCouponNotStarted = errors.New("coupon promotion has not started yet")
	// ErrCouponExpired is returned when a coupon has passed its expiration date.
	ErrCouponExpired = errors.New("coupon has expired")
	// ErrCouponLimitReached is returned when the usage count has reached or exceeded its limit.
	ErrCouponLimitReached = errors.New("coupon usage limit has been reached")
	// ErrCouponMinPurchase is returned when order amount does not reach minimum purchase requirement.
	ErrCouponMinPurchase = errors.New("order total does not meet the minimum purchase amount for this coupon")
	// ErrCouponInvalidCode is returned when coupon code format is invalid.
	ErrCouponInvalidCode = errors.New("invalid coupon code format")
)

// DiscountType defines the discount calculation strategy.
type DiscountType string

const (
	DiscountTypePercentage DiscountType = "percentage"
	DiscountTypeFixed      DiscountType = "fixed"
)

// Coupon represents a promotional discount code.
type Coupon struct {
	ID                int64        `json:"id"`
	Code              string       `json:"code"`
	DiscountType      DiscountType `json:"discount_type"`
	DiscountValue     int64        `json:"discount_value"` // percentage (1-100) or fixed amount in IDR
	MinPurchaseAmount int64        `json:"min_purchase_amount"`
	MaxDiscountAmount *int64       `json:"max_discount_amount,omitempty"` // cap for percentage discount
	UsageLimit        *int         `json:"usage_limit,omitempty"`
	UsedCount         int          `json:"used_count"`
	IsActive          bool         `json:"is_active"`
	StartsAt          *time.Time   `json:"starts_at,omitempty"`
	ExpiresAt         *time.Time   `json:"expires_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// CalculateDiscount calculates the actual discount amount for a given order subtotal.
func (c *Coupon) CalculateDiscount(subtotal int64) int64 {
	if subtotal <= 0 || !c.IsActive {
		return 0
	}
	if subtotal < c.MinPurchaseAmount {
		return 0
	}

	var discount int64
	switch c.DiscountType {
	case DiscountTypePercentage:
		discount = (subtotal * c.DiscountValue) / 100
		if c.MaxDiscountAmount != nil && discount > *c.MaxDiscountAmount {
			discount = *c.MaxDiscountAmount
		}
	case DiscountTypeFixed:
		discount = c.DiscountValue
	default:
		return 0
	}

	// Discount cannot exceed subtotal
	if discount > subtotal {
		discount = subtotal
	}
	return discount
}

// Validate checks whether the coupon is valid for usage at a specific time and order subtotal.
func (c *Coupon) Validate(now time.Time, subtotal int64) error {
	if !c.IsActive {
		return ErrCouponInactive
	}
	if c.StartsAt != nil && now.Before(*c.StartsAt) {
		return ErrCouponNotStarted
	}
	if c.ExpiresAt != nil && now.After(*c.ExpiresAt) {
		return ErrCouponExpired
	}
	if c.UsageLimit != nil && c.UsedCount >= *c.UsageLimit {
		return ErrCouponLimitReached
	}
	if subtotal < c.MinPurchaseAmount {
		return ErrCouponMinPurchase
	}
	return nil
}

// NormalizeCouponCode trims and uppercases a coupon code.
func NormalizeCouponCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// CouponRepository defines persistence operations for coupons.
type CouponRepository interface {
	Create(ctx context.Context, coupon *Coupon) error
	FindByID(ctx context.Context, id int64) (*Coupon, error)
	FindByCode(ctx context.Context, code string) (*Coupon, error)
	ListAll(ctx context.Context, limit, offset int) ([]Coupon, int, error)
	Update(ctx context.Context, coupon *Coupon) error
	IncrementUsedCount(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}

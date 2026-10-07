package domain_test

import (
	"testing"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

func TestUserDomain(t *testing.T) {
	admin := domain.User{Role: domain.RoleAdmin}
	customer := domain.User{Role: domain.RoleCustomer}

	if !admin.IsAdmin() {
		t.Errorf("expected admin.IsAdmin() to be true")
	}

	if customer.IsAdmin() {
		t.Errorf("expected customer.IsAdmin() to be false")
	}
}

func TestProductDomain(t *testing.T) {
	publishedProd := domain.Product{Status: domain.StatusPublished}
	draftProd := domain.Product{Status: domain.StatusDraft}

	if !publishedProd.IsPublished() {
		t.Errorf("expected publishedProd.IsPublished() to be true")
	}

	if draftProd.IsPublished() {
		t.Errorf("expected draftProd.IsPublished() to be false")
	}
}

func TestOrderDomain(t *testing.T) {
	paidOrder := domain.Order{Status: domain.StatusPaid}
	pendingOrder := domain.Order{Status: domain.StatusPending}

	if !paidOrder.IsPaid() {
		t.Errorf("expected paidOrder.IsPaid() to be true")
	}

	if pendingOrder.IsPaid() {
		t.Errorf("expected pendingOrder.IsPaid() to be false")
	}
}

func TestSessionExpiration(t *testing.T) {
	expiredSession := domain.Session{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	activeSession := domain.Session{
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if !expiredSession.IsExpired() {
		t.Errorf("expected session to be expired")
	}

	if activeSession.IsExpired() {
		t.Errorf("expected session to be active (not expired)")
	}
}

func TestCouponDomain(t *testing.T) {
	now := time.Now()

	// 1. Percentage discount calculation
	maxCap := int64(25000)
	percentCoupon := domain.Coupon{
		Code:              "HEMAT20",
		DiscountType:      domain.DiscountTypePercentage,
		DiscountValue:     20, // 20%
		MinPurchaseAmount: 50000,
		MaxDiscountAmount: &maxCap,
		IsActive:          true,
	}

	// Subtotal below min purchase -> 0 discount
	if d := percentCoupon.CalculateDiscount(40000); d != 0 {
		t.Errorf("expected 0 discount below min purchase, got %d", d)
	}

	// Subtotal 100,000 -> 20% is 20,000 (below 25,000 cap) -> 20,000
	if d := percentCoupon.CalculateDiscount(100000); d != 20000 {
		t.Errorf("expected 20000 discount, got %d", d)
	}

	// Subtotal 200,000 -> 20% is 40,000 (capped at 25,000) -> 25,000
	if d := percentCoupon.CalculateDiscount(200000); d != 25000 {
		t.Errorf("expected 25000 discount due to cap, got %d", d)
	}

	// 2. Fixed discount calculation
	fixedCoupon := domain.Coupon{
		Code:          "POTONGAN10K",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 10000,
		IsActive:      true,
	}

	if d := fixedCoupon.CalculateDiscount(50000); d != 10000 {
		t.Errorf("expected 10000 discount, got %d", d)
	}

	// Discount cannot exceed subtotal
	if d := fixedCoupon.CalculateDiscount(8000); d != 8000 {
		t.Errorf("expected discount capped at subtotal (8000), got %d", d)
	}

	// Inactive coupon returns 0
	inactiveCoupon := domain.Coupon{
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 10000,
		IsActive:      false,
	}
	if d := inactiveCoupon.CalculateDiscount(50000); d != 0 {
		t.Errorf("expected 0 discount for inactive coupon, got %d", d)
	}

	// 3. Validation logic
	startsInFuture := now.Add(24 * time.Hour)
	expiredPast := now.Add(-24 * time.Hour)
	limit := 5

	couponFuture := domain.Coupon{IsActive: true, StartsAt: &startsInFuture}
	if err := couponFuture.Validate(now, 100000); err != domain.ErrCouponNotStarted {
		t.Errorf("expected ErrCouponNotStarted, got %v", err)
	}

	couponExpired := domain.Coupon{IsActive: true, ExpiresAt: &expiredPast}
	if err := couponExpired.Validate(now, 100000); err != domain.ErrCouponExpired {
		t.Errorf("expected ErrCouponExpired, got %v", err)
	}

	couponLimitReached := domain.Coupon{IsActive: true, UsageLimit: &limit, UsedCount: 5}
	if err := couponLimitReached.Validate(now, 100000); err != domain.ErrCouponLimitReached {
		t.Errorf("expected ErrCouponLimitReached, got %v", err)
	}

	couponMinPurchase := domain.Coupon{IsActive: true, MinPurchaseAmount: 100000}
	if err := couponMinPurchase.Validate(now, 50000); err != domain.ErrCouponMinPurchase {
		t.Errorf("expected ErrCouponMinPurchase, got %v", err)
	}

	validCoupon := domain.Coupon{IsActive: true, MinPurchaseAmount: 50000}
	if err := validCoupon.Validate(now, 50000); err != nil {
		t.Errorf("expected nil error for valid coupon, got %v", err)
	}

	// 4. Normalize coupon code
	if got := domain.NormalizeCouponCode("  diskon-50  "); got != "DISKON-50" {
		t.Errorf("expected 'DISKON-50', got %q", got)
	}
}

func TestOrderSubtotal(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{Price: 50000},
			{Price: 25000},
		},
	}
	if order.Subtotal() != 75000 {
		t.Errorf("expected Subtotal() = 75000, got %d", order.Subtotal())
	}
}


package coupon_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/coupon"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

type MockCouponRepo struct {
	mu      sync.RWMutex
	coupons map[int64]*domain.Coupon
	byCode  map[string]*domain.Coupon
	nextID  int64
}

func NewMockCouponRepo() *MockCouponRepo {
	return &MockCouponRepo{
		coupons: make(map[int64]*domain.Coupon),
		byCode:  make(map[string]*domain.Coupon),
		nextID:  1,
	}
}

func (m *MockCouponRepo) Create(ctx context.Context, c *domain.Coupon) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	code := domain.NormalizeCouponCode(c.Code)
	if _, exists := m.byCode[code]; exists {
		return domain.ErrConflict
	}
	c.ID = m.nextID
	m.nextID++
	c.Code = code
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	cp := *c
	m.coupons[c.ID] = &cp
	m.byCode[code] = &cp
	return nil
}

func (m *MockCouponRepo) FindByID(ctx context.Context, id int64) (*domain.Coupon, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.coupons[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *MockCouponRepo) FindByCode(ctx context.Context, code string) (*domain.Coupon, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	norm := domain.NormalizeCouponCode(code)
	c, ok := m.byCode[norm]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *MockCouponRepo) ListAll(ctx context.Context, limit, offset int) ([]domain.Coupon, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []domain.Coupon
	for _, c := range m.coupons {
		list = append(list, *c)
	}
	total := len(list)
	if offset > total {
		return []domain.Coupon{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return list[offset:end], total, nil
}

func (m *MockCouponRepo) Update(ctx context.Context, c *domain.Coupon) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.coupons[c.ID]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.byCode, existing.Code)

	c.Code = domain.NormalizeCouponCode(c.Code)
	c.UpdatedAt = time.Now().UTC()
	cp := *c
	m.coupons[c.ID] = &cp
	m.byCode[c.Code] = &cp
	return nil
}

func (m *MockCouponRepo) IncrementUsedCount(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.coupons[id]
	if !ok {
		return domain.ErrNotFound
	}
	c.UsedCount++
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MockCouponRepo) Delete(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.coupons[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.byCode, c.Code)
	delete(m.coupons, id)
	return nil
}

func TestCouponService_ValidateAndCalculate(t *testing.T) {
	repo := NewMockCouponRepo()
	svc := coupon.NewService(repo)
	ctx := context.Background()

	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)
	maxCap := int64(30000)
	limit := 3

	// Setup coupons
	_ = repo.Create(ctx, &domain.Coupon{
		Code:              "DISKON20",
		DiscountType:      domain.DiscountTypePercentage,
		DiscountValue:     20,
		MinPurchaseAmount: 50000,
		MaxDiscountAmount: &maxCap,
		IsActive:          true,
	})

	_ = repo.Create(ctx, &domain.Coupon{
		Code:          "HEMAT10K",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 10000,
		IsActive:      true,
	})

	_ = repo.Create(ctx, &domain.Coupon{
		Code:          "NONAKTIF",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 5000,
		IsActive:      false,
	})

	_ = repo.Create(ctx, &domain.Coupon{
		Code:          "KADALUARSA",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 5000,
		ExpiresAt:     &past,
		IsActive:      true,
	})

	_ = repo.Create(ctx, &domain.Coupon{
		Code:          "BELUMMULAI",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 5000,
		StartsAt:      &future,
		IsActive:      true,
	})

	_ = repo.Create(ctx, &domain.Coupon{
		Code:          "HABIS",
		DiscountType:  domain.DiscountTypeFixed,
		DiscountValue: 5000,
		UsageLimit:    &limit,
		UsedCount:     3,
		IsActive:      true,
	})

	// 1. Success percentage (below cap)
	c, disc, err := svc.ValidateAndCalculate(ctx, "diskon20", 100000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Code != "DISKON20" || disc != 20000 {
		t.Errorf("got coupon %s, disc %d, want DISKON20, 20000", c.Code, disc)
	}

	// 2. Success percentage with cap triggered
	_, discCap, err := svc.ValidateAndCalculate(ctx, "DISKON20", 200000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if discCap != 30000 {
		t.Errorf("got disc %d, want 30000 (cap)", discCap)
	}

	// 3. Success fixed
	_, discFixed, err := svc.ValidateAndCalculate(ctx, "HEMAT10K", 50000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if discFixed != 10000 {
		t.Errorf("got disc %d, want 10000", discFixed)
	}

	// 4. Minimum purchase not met
	_, _, err = svc.ValidateAndCalculate(ctx, "DISKON20", 30000)
	if !errors.Is(err, domain.ErrCouponMinPurchase) {
		t.Errorf("expected ErrCouponMinPurchase, got %v", err)
	}

	// 5. Inactive coupon
	_, _, err = svc.ValidateAndCalculate(ctx, "NONAKTIF", 50000)
	if !errors.Is(err, domain.ErrCouponInactive) {
		t.Errorf("expected ErrCouponInactive, got %v", err)
	}

	// 6. Expired coupon
	_, _, err = svc.ValidateAndCalculate(ctx, "KADALUARSA", 50000)
	if !errors.Is(err, domain.ErrCouponExpired) {
		t.Errorf("expected ErrCouponExpired, got %v", err)
	}

	// 7. Future coupon
	_, _, err = svc.ValidateAndCalculate(ctx, "BELUMMULAI", 50000)
	if !errors.Is(err, domain.ErrCouponNotStarted) {
		t.Errorf("expected ErrCouponNotStarted, got %v", err)
	}

	// 8. Limit reached coupon
	_, _, err = svc.ValidateAndCalculate(ctx, "HABIS", 50000)
	if !errors.Is(err, domain.ErrCouponLimitReached) {
		t.Errorf("expected ErrCouponLimitReached, got %v", err)
	}

	// 9. Non-existent code
	_, _, err = svc.ValidateAndCalculate(ctx, "TIDAKADA", 50000)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// 10. Empty code
	_, _, err = svc.ValidateAndCalculate(ctx, "   ", 50000)
	if !errors.Is(err, domain.ErrCouponInvalidCode) {
		t.Errorf("expected ErrCouponInvalidCode, got %v", err)
	}
}

func TestCouponService_CRUD(t *testing.T) {
	repo := NewMockCouponRepo()
	svc := coupon.NewService(repo)
	ctx := context.Background()

	// 1. Create Coupon
	created, err := svc.CreateCoupon(ctx, coupon.CreateCouponInput{
		Code:              "PROMO50",
		DiscountType:      domain.DiscountTypePercentage,
		DiscountValue:     50,
		MinPurchaseAmount: 100000,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("failed to create coupon: %v", err)
	}
	if created.ID == 0 || created.Code != "PROMO50" {
		t.Errorf("unexpected created coupon: %+v", created)
	}

	// Validation errors
	_, err = svc.CreateCoupon(ctx, coupon.CreateCouponInput{Code: "", DiscountType: domain.DiscountTypeFixed, DiscountValue: 1000})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty code, got %v", err)
	}

	_, err = svc.CreateCoupon(ctx, coupon.CreateCouponInput{Code: "FAIL", DiscountType: "invalid_type", DiscountValue: 1000})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for invalid type, got %v", err)
	}

	_, err = svc.CreateCoupon(ctx, coupon.CreateCouponInput{Code: "FAIL", DiscountType: domain.DiscountTypePercentage, DiscountValue: 105})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for percent > 100, got %v", err)
	}

	_, err = svc.CreateCoupon(ctx, coupon.CreateCouponInput{Code: "FAIL", DiscountType: domain.DiscountTypeFixed, DiscountValue: 1000, MinPurchaseAmount: -500})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for negative min purchase, got %v", err)
	}

	negCap := int64(-1000)
	_, err = svc.CreateCoupon(ctx, coupon.CreateCouponInput{Code: "FAIL", DiscountType: domain.DiscountTypePercentage, DiscountValue: 20, MaxDiscountAmount: &negCap})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for negative max discount cap, got %v", err)
	}

	negLimit := -5
	_, err = svc.CreateCoupon(ctx, coupon.CreateCouponInput{Code: "FAIL", DiscountType: domain.DiscountTypePercentage, DiscountValue: 20, UsageLimit: &negLimit})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for negative usage limit, got %v", err)
	}

	// 2. Get Coupon
	found, err := svc.GetCoupon(ctx, created.ID)
	if err != nil || found.Code != "PROMO50" {
		t.Fatalf("failed to get coupon: %v", err)
	}

	// 3. Update Coupon
	updated, err := svc.UpdateCoupon(ctx, created.ID, coupon.UpdateCouponInput{
		Code:              "PROMO60",
		DiscountType:      domain.DiscountTypePercentage,
		DiscountValue:     60,
		MinPurchaseAmount: 120000,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("failed to update coupon: %v", err)
	}
	if updated.Code != "PROMO60" || updated.DiscountValue != 60 {
		t.Errorf("unexpected updated coupon: %+v", updated)
	}

	// 4. Toggle Status
	toggled, err := svc.ToggleCouponStatus(ctx, created.ID)
	if err != nil || toggled.IsActive {
		t.Errorf("expected coupon to be toggled to inactive, got %v (isActive: %v)", err, toggled.IsActive)
	}

	// 5. Delete Coupon
	if err := svc.DeleteCoupon(ctx, created.ID); err != nil {
		t.Fatalf("failed to delete coupon: %v", err)
	}

	_, err = svc.GetCoupon(ctx, created.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

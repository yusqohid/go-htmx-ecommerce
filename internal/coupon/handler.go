package coupon

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

// Handler handles administrative HTTP requests for coupon promotions.
type Handler struct {
	service *Service
	view    *view.View
}

// NewHandler constructs a new coupon Handler.
func NewHandler(service *Service, view *view.View) *Handler {
	return &Handler{
		service: service,
		view:    view,
	}
}

// ListCoupons renders the admin table of all promotional coupons.
func (h *Handler) ListCoupons(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	coupons, total, _ := h.service.ListCoupons(r.Context(), page, 50)

	_ = h.view.Render(w, "admin", "admin/coupons/index", map[string]any{
		"Title":     "Coupons & Discounts",
		"ActiveNav": "coupons",
		"User":      user,
		"Coupons":   coupons,
		"Total":     total,
		"Page":      page,
	})
}

// NewCoupon renders the blank coupon creation form.
func (h *Handler) NewCoupon(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	_ = h.view.Render(w, "admin", "admin/coupons/form", map[string]any{
		"Title":     "New Coupon",
		"ActiveNav": "coupons",
		"User":      user,
		"IsEdit":    false,
		"Coupon": &domain.Coupon{
			IsActive:     true,
			DiscountType: domain.DiscountTypePercentage,
		},
		"Error": "",
	})
}

// CreateCoupon processes the submission of a new coupon.
func (h *Handler) CreateCoupon(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	user := auth.UserFromContext(r.Context())
	input, err := parseCouponForm(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = h.view.Render(w, "admin", "admin/coupons/form", map[string]any{
			"Title":     "New Coupon",
			"ActiveNav": "coupons",
			"User":      user,
			"IsEdit":    false,
			"Coupon": &domain.Coupon{
				Code:              r.FormValue("code"),
				DiscountType:      domain.DiscountType(r.FormValue("discount_type")),
				DiscountValue:     input.DiscountValue,
				MinPurchaseAmount: input.MinPurchaseAmount,
				MaxDiscountAmount: input.MaxDiscountAmount,
				UsageLimit:        input.UsageLimit,
				IsActive:          input.IsActive,
				StartsAt:          input.StartsAt,
				ExpiresAt:         input.ExpiresAt,
			},
			"Error": err.Error(),
		})
		return
	}

	_, err = h.service.CreateCoupon(r.Context(), input)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = h.view.Render(w, "admin", "admin/coupons/form", map[string]any{
			"Title":     "New Coupon",
			"ActiveNav": "coupons",
			"User":      user,
			"IsEdit":    false,
			"Coupon": &domain.Coupon{
				Code:              r.FormValue("code"),
				DiscountType:      domain.DiscountType(r.FormValue("discount_type")),
				DiscountValue:     input.DiscountValue,
				MinPurchaseAmount: input.MinPurchaseAmount,
				MaxDiscountAmount: input.MaxDiscountAmount,
				UsageLimit:        input.UsageLimit,
				IsActive:          input.IsActive,
				StartsAt:          input.StartsAt,
				ExpiresAt:         input.ExpiresAt,
			},
			"Error": err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/admin/coupons", http.StatusSeeOther)
}

// EditCoupon renders the edit form for an existing coupon.
func (h *Handler) EditCoupon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	c, err := h.service.GetCoupon(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user := auth.UserFromContext(r.Context())
	_ = h.view.Render(w, "admin", "admin/coupons/form", map[string]any{
		"Title":     "Edit Coupon - " + c.Code,
		"ActiveNav": "coupons",
		"User":      user,
		"IsEdit":    true,
		"Coupon":    c,
		"Error":     "",
	})
}

// UpdateCoupon processes updates to an existing coupon.
func (h *Handler) UpdateCoupon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	user := auth.UserFromContext(r.Context())
	input, err := parseCouponForm(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = h.view.Render(w, "admin", "admin/coupons/form", map[string]any{
			"Title":     "Edit Coupon",
			"ActiveNav": "coupons",
			"User":      user,
			"IsEdit":    true,
			"Coupon": &domain.Coupon{
				ID:                id,
				Code:              r.FormValue("code"),
				DiscountType:      domain.DiscountType(r.FormValue("discount_type")),
				DiscountValue:     input.DiscountValue,
				MinPurchaseAmount: input.MinPurchaseAmount,
				MaxDiscountAmount: input.MaxDiscountAmount,
				UsageLimit:        input.UsageLimit,
				IsActive:          input.IsActive,
				StartsAt:          input.StartsAt,
				ExpiresAt:         input.ExpiresAt,
			},
			"Error": err.Error(),
		})
		return
	}

	updateInput := UpdateCouponInput{
		Code:              input.Code,
		DiscountType:      input.DiscountType,
		DiscountValue:     input.DiscountValue,
		MinPurchaseAmount: input.MinPurchaseAmount,
		MaxDiscountAmount: input.MaxDiscountAmount,
		UsageLimit:        input.UsageLimit,
		IsActive:          input.IsActive,
		StartsAt:          input.StartsAt,
		ExpiresAt:         input.ExpiresAt,
	}

	_, err = h.service.UpdateCoupon(r.Context(), id, updateInput)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = h.view.Render(w, "admin", "admin/coupons/form", map[string]any{
			"Title":     "Edit Coupon",
			"ActiveNav": "coupons",
			"User":      user,
			"IsEdit":    true,
			"Coupon": &domain.Coupon{
				ID:                id,
				Code:              r.FormValue("code"),
				DiscountType:      domain.DiscountType(r.FormValue("discount_type")),
				DiscountValue:     input.DiscountValue,
				MinPurchaseAmount: input.MinPurchaseAmount,
				MaxDiscountAmount: input.MaxDiscountAmount,
				UsageLimit:        input.UsageLimit,
				IsActive:          input.IsActive,
				StartsAt:          input.StartsAt,
				ExpiresAt:         input.ExpiresAt,
			},
			"Error": err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/admin/coupons", http.StatusSeeOther)
}

// ToggleStatus handles inline HTMX toggling between Active and Inactive.
func (h *Handler) ToggleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	c, err := h.service.ToggleCouponStatus(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if c.IsActive {
		fmt.Fprintf(w, `<div id="status-badge-%d"><span class="badge-table success" style="cursor: pointer;" hx-post="/admin/coupons/%d/toggle-status" hx-target="#status-badge-%d" hx-swap="outerHTML" title="Click to Deactivate">Active</span></div>`, c.ID, c.ID, c.ID)
	} else {
		fmt.Fprintf(w, `<div id="status-badge-%d"><span class="badge-table pending" style="cursor: pointer;" hx-post="/admin/coupons/%d/toggle-status" hx-target="#status-badge-%d" hx-swap="outerHTML" title="Click to Activate">Inactive</span></div>`, c.ID, c.ID, c.ID)
	}
}

// DeleteCoupon handles coupon deletion.
func (h *Handler) DeleteCoupon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.service.DeleteCoupon(r.Context(), id); err != nil && !errors.Is(err, domain.ErrNotFound) {
		log.Printf("[CouponHandler] Failed to delete coupon %d: %v", id, err)
		http.Error(w, fmt.Sprintf("Failed to delete coupon: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/coupons", http.StatusSeeOther)
}

func parseCouponForm(r *http.Request) (CreateCouponInput, error) {
	code := domain.NormalizeCouponCode(r.FormValue("code"))
	if code == "" {
		return CreateCouponInput{}, fmt.Errorf("coupon code is required")
	}

	discountType := domain.DiscountType(r.FormValue("discount_type"))
	if discountType != domain.DiscountTypePercentage && discountType != domain.DiscountTypeFixed {
		return CreateCouponInput{}, fmt.Errorf("valid discount type is required (percentage or fixed)")
	}

	discountValue, err := strconv.ParseInt(r.FormValue("discount_value"), 10, 64)
	if err != nil || discountValue <= 0 {
		return CreateCouponInput{}, fmt.Errorf("discount value must be a positive number")
	}

	minPurchase, _ := strconv.ParseInt(r.FormValue("min_purchase_amount"), 10, 64)
	if minPurchase < 0 {
		minPurchase = 0
	}

	var maxDiscount *int64
	if val := strings.TrimSpace(r.FormValue("max_discount_amount")); val != "" {
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err == nil && parsed > 0 {
			maxDiscount = &parsed
		}
	}

	var usageLimit *int
	if val := strings.TrimSpace(r.FormValue("usage_limit")); val != "" {
		parsed, err := strconv.Atoi(val)
		if err == nil && parsed > 0 {
			usageLimit = &parsed
		}
	}

	isActive := r.FormValue("is_active") == "on" || r.FormValue("is_active") == "true" || r.FormValue("is_active") == "1"
	startsAt := parseDateTimeLocal(r.FormValue("starts_at"))
	expiresAt := parseDateTimeLocal(r.FormValue("expires_at"))

	return CreateCouponInput{
		Code:              code,
		DiscountType:      discountType,
		DiscountValue:     discountValue,
		MinPurchaseAmount: minPurchase,
		MaxDiscountAmount: maxDiscount,
		UsageLimit:        usageLimit,
		IsActive:          isActive,
		StartsAt:          startsAt,
		ExpiresAt:         expiresAt,
	}, nil
}

func parseDateTimeLocal(val string) *time.Time {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02T15:04", trimmed)
	if err != nil {
		t, err = time.Parse(time.RFC3339, trimmed)
		if err != nil {
			return nil
		}
	}
	utc := t.UTC()
	return &utc
}

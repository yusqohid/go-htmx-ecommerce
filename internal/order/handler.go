package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/coupon"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

// Handler handles HTTP requests for checkout, order placement, and mock payment simulations.
type Handler struct {
	orderService    *Service
	productRepo     domain.ProductRepository
	couponService   *coupon.Service
	view            *view.View
	paymentProvider string
	clientKey       string
	snapScriptURL   string
}

// NewHandler constructs a new order Handler.
func NewHandler(
	orderService *Service,
	productRepo domain.ProductRepository,
	couponService *coupon.Service,
	view *view.View,
	paymentProvider string,
	clientKey string,
	snapScriptURL string,
) *Handler {
	return &Handler{
		orderService:    orderService,
		productRepo:     productRepo,
		couponService:   couponService,
		view:            view,
		paymentProvider: paymentProvider,
		clientKey:       clientKey,
		snapScriptURL:   snapScriptURL,
	}
}

// CheckoutPage renders the order review and confirmation screen before payment redirection.
func (h *Handler) CheckoutPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
		return
	}

	productIDStr := chi.URLParam(r, "productID")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	product, err := h.productRepo.FindByID(r.Context(), productID)
	if err != nil {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	if !product.IsPublished() {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	// Check if customer already owns this product
	alreadyOwns, _ := h.orderService.HasAccessToProduct(r.Context(), user.ID, product.ID)
	if alreadyOwns {
		_ = h.view.Render(w, "public", "storefront/checkout", map[string]any{
			"Title":           "Checkout - " + product.Name,
			"User":            user,
			"Product":         product,
			"AlreadyOwned":    true,
			"PaymentProvider": strings.ToUpper(h.paymentProvider),
			"ClientKey":       h.clientKey,
			"SnapScriptURL":   h.snapScriptURL,
		})
		return
	}

	couponCode := strings.TrimSpace(r.URL.Query().Get("coupon"))
	var appliedCoupon *domain.Coupon
	var discountAmount int64
	var couponError string
	var couponSuccess string
	totalAmount := product.Price

	if couponCode != "" && h.couponService != nil {
		c, disc, err := h.couponService.ValidateAndCalculate(r.Context(), couponCode, product.Price)
		if err != nil {
			couponError = translateCouponError(err)
		} else {
			appliedCoupon = c
			discountAmount = disc
			totalAmount = product.Price - disc
			couponSuccess = fmt.Sprintf("Kupon %s berhasil diterapkan!", c.Code)
		}
	}

	_ = h.view.Render(w, "public", "storefront/checkout", map[string]any{
		"Title":           "Checkout - " + product.Name,
		"User":            user,
		"Product":         product,
		"PaymentProvider": strings.ToUpper(h.paymentProvider),
		"ClientKey":       h.clientKey,
		"SnapScriptURL":   h.snapScriptURL,
		"Coupon":          appliedCoupon,
		"CouponCode":      couponCode,
		"DiscountAmount":  discountAmount,
		"TotalAmount":     totalAmount,
		"CouponError":     couponError,
		"CouponSuccess":   couponSuccess,
	})
}

// ApplyCoupon validates a coupon code and renders the updated checkout summary via HTMX.
func (h *Handler) ApplyCoupon(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
		return
	}

	productIDStr := chi.URLParam(r, "productID")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	product, err := h.productRepo.FindByID(r.Context(), productID)
	if err != nil || !product.IsPublished() {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	couponCode := strings.TrimSpace(r.FormValue("coupon_code"))
	var appliedCoupon *domain.Coupon
	var discountAmount int64
	var couponError string
	var couponSuccess string
	totalAmount := product.Price

	if couponCode == "" {
		couponError = "Silakan masukkan kode kupon."
	} else if h.couponService != nil {
		c, disc, err := h.couponService.ValidateAndCalculate(r.Context(), couponCode, product.Price)
		if err != nil {
			couponError = translateCouponError(err)
		} else {
			appliedCoupon = c
			discountAmount = disc
			totalAmount = product.Price - disc
			couponSuccess = fmt.Sprintf("Kupon %s berhasil diterapkan!", c.Code)
		}
	}

	_ = h.view.Render(w, "public", "storefront/checkout", map[string]any{
		"Title":           "Checkout - " + product.Name,
		"User":            user,
		"Product":         product,
		"PaymentProvider": strings.ToUpper(h.paymentProvider),
		"ClientKey":       h.clientKey,
		"SnapScriptURL":   h.snapScriptURL,
		"Coupon":          appliedCoupon,
		"CouponCode":      couponCode,
		"DiscountAmount":  discountAmount,
		"TotalAmount":     totalAmount,
		"CouponError":     couponError,
		"CouponSuccess":   couponSuccess,
	})
}

// RemoveCoupon removes the applied coupon and renders the updated checkout summary via HTMX.
func (h *Handler) RemoveCoupon(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
		return
	}

	productIDStr := chi.URLParam(r, "productID")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	product, err := h.productRepo.FindByID(r.Context(), productID)
	if err != nil || !product.IsPublished() {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	_ = h.view.Render(w, "public", "storefront/checkout", map[string]any{
		"Title":           "Checkout - " + product.Name,
		"User":            user,
		"Product":         product,
		"PaymentProvider": strings.ToUpper(h.paymentProvider),
		"ClientKey":       h.clientKey,
		"SnapScriptURL":   h.snapScriptURL,
		"Coupon":          nil,
		"CouponCode":      "",
		"DiscountAmount":  0,
		"TotalAmount":     product.Price,
	})
}

// ProcessCheckout creates the order and redirects the user to the payment provider's checkout session.
func (h *Handler) ProcessCheckout(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	productIDStr := chi.URLParam(r, "productID")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/products", http.StatusSeeOther)
		return
	}

	couponCode := strings.TrimSpace(r.FormValue("coupon_code"))

	order, checkoutURL, err := h.orderService.CreateOrder(r.Context(), CreateOrderInput{
		Customer:   user,
		ProductID:  productID,
		CouponCode: couponCode,
	})
	if err != nil {
		errMsg := "Gagal memproses pesanan. Silakan coba lagi."
		if errors.Is(err, ErrAlreadyPurchased) {
			errMsg = "Anda sudah pernah membeli produk digital ini."
		} else if errors.Is(err, ErrProductNotAvailable) {
			errMsg = "Produk ini saat ini tidak tersedia."
		} else if errors.Is(err, domain.ErrCouponNotFound) {
			errMsg = "Kode kupon tidak ditemukan."
		} else if errors.Is(err, domain.ErrCouponInactive) {
			errMsg = "Kupon saat ini sedang tidak aktif."
		} else if errors.Is(err, domain.ErrCouponExpired) {
			errMsg = "Kupon telah kedaluwarsa."
		} else if errors.Is(err, domain.ErrCouponNotStarted) {
			errMsg = "Promo kupon belum dimulai."
		} else if errors.Is(err, domain.ErrCouponLimitReached) {
			errMsg = "Kuota pemakaian kupon telah habis."
		} else if errors.Is(err, domain.ErrCouponMinPurchase) {
			errMsg = "Total belanja belum memenuhi minimum pembelian kupon ini."
		}

		product, fetchErr := h.productRepo.FindByID(r.Context(), productID)
		if fetchErr != nil {
			http.Redirect(w, r, "/products", http.StatusSeeOther)
			return
		}

		_ = h.view.Render(w, "public", "storefront/checkout", map[string]any{
			"Title":           "Checkout - " + product.Name,
			"User":            user,
			"Product":         product,
			"Error":           errMsg,
			"PaymentProvider": strings.ToUpper(h.paymentProvider),
			"ClientKey":       h.clientKey,
			"SnapScriptURL":   h.snapScriptURL,
			"CouponCode":      couponCode,
			"TotalAmount":     product.Price,
		})
		return
	}

	_ = order
	// Return JSON response for AJAX / HTMX requests to allow in-page Snap modal payment
	if r.Header.Get("HX-Request") == "true" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		token := ""
		parts := strings.Split(strings.TrimRight(checkoutURL, "/"), "/")
		if len(parts) > 0 {
			token = parts[len(parts)-1]
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order_reference": order.Reference,
			"checkout_url":    checkoutURL,
			"snap_token":      token,
		})
		return
	}

	// Normal browser submission fallback: redirect to external checkout page
	http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
}

func translateCouponError(err error) string {
	if errors.Is(err, domain.ErrCouponNotFound) || errors.Is(err, domain.ErrNotFound) {
		return "Kode kupon tidak ditemukan."
	}
	if errors.Is(err, domain.ErrCouponInactive) {
		return "Kupon saat ini sedang tidak aktif."
	}
	if errors.Is(err, domain.ErrCouponExpired) {
		return "Kupon telah kedaluwarsa."
	}
	if errors.Is(err, domain.ErrCouponNotStarted) {
		return "Promo kupon belum dimulai."
	}
	if errors.Is(err, domain.ErrCouponLimitReached) {
		return "Batas kuota pemakaian kupon telah tercapai."
	}
	if errors.Is(err, domain.ErrCouponMinPurchase) {
		return "Total belanja belum memenuhi minimum pembelian untuk kupon ini."
	}
	if errors.Is(err, domain.ErrCouponInvalidCode) {
		return "Format kode kupon tidak valid."
	}
	return "Kupon tidak dapat digunakan."
}

// OrderSuccess renders order summary and status details after checkout redirection.
func (h *Handler) OrderSuccess(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	ref := chi.URLParam(r, "reference")

	// Attempt real-time payment gateway status synchronization if order is pending
	order, err := h.orderService.SyncPaymentStatus(r.Context(), ref)
	if err != nil || order == nil {
		order, err = h.orderService.GetOrderByReference(r.Context(), ref)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	// Ownership check: only the order's customer may view the receipt.
	if order.CustomerID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_ = h.view.Render(w, "public", "storefront/order_success", map[string]any{
		"Title": "Order " + order.Reference + " - Sellora",
		"User":  user,
		"Order": order,
	})
}

// MockCheckoutPage displays the development simulation payment screen.
func (h *Handler) MockCheckoutPage(w http.ResponseWriter, r *http.Request) {
	if h.paymentProvider != "mock" {
		http.Error(w, "Mock checkout disabled", http.StatusNotFound)
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	ref := r.URL.Query().Get("ref")
	if ref == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	order, err := h.orderService.GetOrderByReference(r.Context(), ref)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Ownership check: only the order's customer may access the sandbox.
	if order.CustomerID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_ = h.view.Render(w, "public", "storefront/mock_checkout", map[string]any{
		"Title": "Mock Sandbox Payment Gateway",
		"Order": order,
	})
}

// MockSimulatePayment processes the simulated payment action in development mode.
func (h *Handler) MockSimulatePayment(w http.ResponseWriter, r *http.Request) {
	if h.paymentProvider != "mock" {
		http.Error(w, "Mock simulation disabled", http.StatusNotFound)
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	ref := r.FormValue("ref")
	simulatedStatus := r.FormValue("status")

	order, err := h.orderService.GetOrderByReference(r.Context(), ref)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Ownership check: only the order's customer may simulate payment.
	if order.CustomerID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var status domain.OrderStatus
	switch simulatedStatus {
	case "paid":
		status = domain.StatusPaid
	case "failed":
		status = domain.StatusFailed
	default:
		http.Error(w, "Invalid status value. Must be 'paid' or 'failed'.", http.StatusBadRequest)
		return
	}

	paymentRef := fmt.Sprintf("MOCK-TX-%d", order.ID)
	if err := h.orderService.UpdateOrderStatus(r.Context(), order.ID, status, paymentRef); err != nil {
		http.Error(w, "Failed to update order status", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/orders/%s/success", order.Reference), http.StatusSeeOther)
}

package order

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

// Handler handles HTTP requests for checkout, order placement, and mock payment simulations.
type Handler struct {
	orderService    *Service
	productRepo     domain.ProductRepository
	view            *view.View
	paymentProvider string
}

// NewHandler constructs a new order Handler.
func NewHandler(
	orderService *Service,
	productRepo domain.ProductRepository,
	view *view.View,
	paymentProvider string,
) *Handler {
	return &Handler{
		orderService:    orderService,
		productRepo:     productRepo,
		view:            view,
		paymentProvider: paymentProvider,
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
		})
		return
	}

	_ = h.view.Render(w, "public", "storefront/checkout", map[string]any{
		"Title":           "Checkout - " + product.Name,
		"User":            user,
		"Product":         product,
		"PaymentProvider": strings.ToUpper(h.paymentProvider),
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

	order, checkoutURL, err := h.orderService.CreateOrder(r.Context(), CreateOrderInput{
		Customer:  user,
		ProductID: productID,
	})
	if err != nil {
		errMsg := "Unable to process order. Please try again."
		if errors.Is(err, ErrAlreadyPurchased) {
			errMsg = "You have already purchased this digital product."
		} else if errors.Is(err, ErrProductNotAvailable) {
			errMsg = "This product is currently unavailable."
		}

		product, fetchErr := h.productRepo.FindByID(r.Context(), productID)
		if fetchErr != nil {
			http.Redirect(w, r, "/products", http.StatusSeeOther)
			return
		}

		_ = h.view.Render(w, "public", "storefront/checkout", map[string]any{
			"Title":           "Checkout",
			"User":            user,
			"Product":         product,
			"Error":           errMsg,
			"PaymentProvider": strings.ToUpper(h.paymentProvider),
		})
		return
	}

	_ = order
	// Redirect user to payment provider checkout page
	http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
}

// OrderSuccess renders order summary and status details after checkout redirection.
func (h *Handler) OrderSuccess(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	ref := chi.URLParam(r, "reference")

	order, err := h.orderService.GetOrderByReference(r.Context(), ref)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
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

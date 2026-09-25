package customer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)
// OrderService defines order operations needed by the customer portal.
type OrderService interface {
	ListCustomerOrders(ctx context.Context, customerID int64) ([]domain.Order, error)
	GetOrderByReference(ctx context.Context, ref string) (*domain.Order, error)
	GetOrder(ctx context.Context, id int64, requestingUserID int64, isAdmin bool) (*domain.Order, error)
	SyncPaymentStatus(ctx context.Context, orderReference string) (*domain.Order, error)
}

// AuthService defines authentication and password operations needed by the customer portal.
type AuthService interface {
	ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error
}

// Handler handles customer account dashboard and order history requests.
type Handler struct {
	orderService OrderService
	fileRepo     domain.ProductFileRepository
	authService  AuthService
	view         *view.View
}

// NewHandler constructs a new customer Handler.
func NewHandler(
	orderService OrderService,
	fileRepo domain.ProductFileRepository,
	authService AuthService,
	view *view.View,
) *Handler {
	return &Handler{
		orderService: orderService,
		fileRepo:     fileRepo,
		authService:  authService,
		view:         view,
	}
}
// AccountHome redirects the customer to their order history page.
func (h *Handler) AccountHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/account/orders", http.StatusSeeOther)
}

// ListOrders renders the customer's purchase history and download links for paid products.
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
		return
	}

	orders, err := h.orderService.ListCustomerOrders(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "Failed to load orders", http.StatusInternalServerError)
		return
	}

	// Fetch downloadable files for all items in paid orders
	productFiles := make(map[int64][]domain.ProductFile)
	for _, ord := range orders {
		if ord.Status == domain.StatusPaid {
			for _, item := range ord.Items {
				if _, exists := productFiles[item.ProductID]; !exists {
					files, err := h.fileRepo.FindByProductID(r.Context(), item.ProductID)
					if err == nil {
						productFiles[item.ProductID] = files
					}
				}
			}
		}
	}

	_ = h.view.Render(w, "public", "customer/orders", map[string]any{
		"Title":        "My Purchases & Orders - Sellora",
		"User":         user,
		"Orders":       orders,
		"ProductFiles": productFiles,
		"ActiveNav":    "orders",
	})
}

// OrderDetail renders details and downloads for a specific customer order.
func (h *Handler) OrderDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
		return
	}

	idParam := chi.URLParam(r, "id")
	var order *domain.Order
	var err error

	// Support looking up either by numeric ID or by reference string
	if numID, parseErr := strconv.ParseInt(idParam, 10, 64); parseErr == nil {
		order, err = h.orderService.GetOrder(r.Context(), numID, user.ID, user.IsAdmin())
	} else {
		order, err = h.orderService.GetOrderByReference(r.Context(), idParam)
		if err == nil && order.CustomerID != user.ID && !user.IsAdmin() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
	if err != nil {
		http.Redirect(w, r, "/account/orders", http.StatusSeeOther)
		return
	}

	// If order is still pending, attempt real-time sync with payment provider
	if order.Status == domain.StatusPending {
		if synced, syncErr := h.orderService.SyncPaymentStatus(r.Context(), order.Reference); syncErr == nil && synced != nil {
			order = synced
		}
	}
	// Load files for items if order is paid
	productFiles := make(map[int64][]domain.ProductFile)
	if order.Status == domain.StatusPaid {
		for _, item := range order.Items {
			files, err := h.fileRepo.FindByProductID(r.Context(), item.ProductID)
			if err == nil {
				productFiles[item.ProductID] = files
			}
		}
	}

	_ = h.view.Render(w, "public", "customer/order_detail", map[string]any{
		"Title":        "Order " + order.Reference + " - Sellora",
		"User":         user,
		"Order":        order,
		"ProductFiles": productFiles,
		"ActiveNav":    "orders",
	})
}
// Settings renders the customer account details and password change form.
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
		return
	}

	_ = h.view.Render(w, "public", "customer/settings", map[string]any{
		"Title":     "Account Settings - Sellora",
		"User":      user,
		"ActiveNav": "settings",
	})
}

// UpdatePassword processes the user's password change request.
func (h *Handler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderPasswordResult(w, r, user, "", "Invalid form submission.")
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if currentPassword == "" || newPassword == "" {
		h.renderPasswordResult(w, r, user, "", "Current and new passwords are required.")
		return
	}

	if newPassword != confirmPassword {
		h.renderPasswordResult(w, r, user, "", "New passwords do not match.")
		return
	}

	if len(newPassword) < 8 {
		h.renderPasswordResult(w, r, user, "", "New password must be at least 8 characters long.")
		return
	}

	if h.authService != nil {
		if err := h.authService.ChangePassword(r.Context(), user.ID, currentPassword, newPassword); err != nil {
			if errors.Is(err, domain.ErrUnauthorized) {
				h.renderPasswordResult(w, r, user, "", "Current password is incorrect.")
				return
			}
			h.renderPasswordResult(w, r, user, "", "Failed to update password. Please try again.")
			return
		}
	}

	h.renderPasswordResult(w, r, user, "Your password has been changed successfully.", "")
}

func (h *Handler) renderPasswordResult(w http.ResponseWriter, r *http.Request, user *domain.User, successMsg, errorMsg string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if errorMsg != "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, `<div class="alert alert-danger rounded-3 d-flex align-items-center gap-2 mb-0"><i class="bi bi-exclamation-triangle-fill"></i><div>%s</div></div>`, errorMsg)
			return
		}
		fmt.Fprintf(w, `<div class="alert alert-success rounded-3 d-flex align-items-center gap-2 mb-0"><i class="bi bi-check-circle-fill"></i><div>%s</div></div>`, successMsg)
		return
	}

	_ = h.view.Render(w, "public", "customer/settings", map[string]any{
		"Title":     "Account Settings - Sellora",
		"User":      user,
		"ActiveNav": "settings",
		"Success":   successMsg,
		"Error":     errorMsg,
	})
}

package order

import (
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

// AdminHandler handles admin back-office order management requests.
type AdminHandler struct {
	service *Service
	view    *view.View
}

// NewAdminHandler constructs a new AdminHandler.
func NewAdminHandler(service *Service, view *view.View) *AdminHandler {
	return &AdminHandler{
		service: service,
		view:    view,
	}
}

// ListOrders renders the admin order dashboard with pagination.
func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 20
	orders, total, err := h.service.ListAllOrders(r.Context(), page, pageSize)
	if err != nil {
		http.Error(w, "Failed to retrieve orders", http.StatusInternalServerError)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages < 1 {
		totalPages = 1
	}

	_ = h.view.Render(w, "admin", "admin/orders/index", map[string]any{
		"Title":      "Orders Management - Sellora Admin",
		"User":       user,
		"Orders":     orders,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
		"ActiveNav":  "orders",
	})
}

// OrderDetail renders complete order information, items, customer, and gateway reference.
func (h *AdminHandler) OrderDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
		return
	}

	order, err := h.service.GetOrder(r.Context(), id, user.ID, true)
	if err != nil {
		http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
		return
	}

	_ = h.view.Render(w, "admin", "admin/orders/detail", map[string]any{
		"Title":     "Order " + order.Reference + " - Sellora Admin",
		"User":      user,
		"Order":     order,
		"ActiveNav": "orders",
	})
}

package customer

import (
	"context"
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
}

// Handler handles customer account dashboard and order history requests.
type Handler struct {
	orderService OrderService
	fileRepo     domain.ProductFileRepository
	view         *view.View
}

// NewHandler constructs a new customer Handler.
func NewHandler(
	orderService OrderService,
	fileRepo domain.ProductFileRepository,
	view *view.View,
) *Handler {
	return &Handler{
		orderService: orderService,
		fileRepo:     fileRepo,
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

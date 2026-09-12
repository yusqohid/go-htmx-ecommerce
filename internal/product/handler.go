package product

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

// Handler handles HTTP requests for admin product management.
type Handler struct {
	service         *Service
	productRepo     *PostgresProductRepository
	view            *view.View
	paymentProvider string
}

// NewHandler creates a new admin product Handler.
func NewHandler(
	service *Service,
	productRepo *PostgresProductRepository,
	view *view.View,
	paymentProvider string,
) *Handler {
	return &Handler{
		service:         service,
		productRepo:     productRepo,
		view:            view,
		paymentProvider: paymentProvider,
	}
}

// Dashboard renders the store management overview.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	var total, published, drafts int
	if h.productRepo != nil {
		total, published, drafts, _ = h.productRepo.CountStats(r.Context())
	}

	var recentProducts []domain.Product
	if h.service != nil {
		recentProducts, _, _ = h.service.ListProducts(r.Context(), "", "all", 1, 5)
	}

	_ = h.view.Render(w, "admin", "admin/dashboard", map[string]any{
		"Title":             "Dashboard",
		"ActiveNav":         "dashboard",
		"User":              user,
		"TotalProducts":     total,
		"PublishedProducts": published,
		"DraftProducts":     drafts,
		"PaymentProvider":   h.paymentProvider,
		"RecentProducts":    recentProducts,
	})
}

// ListProducts renders the product catalog management table.
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	search := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}

	var products []domain.Product
	var total int
	if h.service != nil {
		products, total, _ = h.service.ListProducts(r.Context(), search, status, 1, 50)
	}

	_ = h.view.Render(w, "admin", "admin/products/index", map[string]any{
		"Title":        "Products",
		"ActiveNav":    "products",
		"User":         user,
		"Products":     products,
		"Total":        total,
		"Search":       search,
		"StatusFilter": status,
	})
}

// NewProduct renders the create product form.
func (h *Handler) NewProduct(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	_ = h.view.Render(w, "admin", "admin/products/form", map[string]any{
		"Title":     "New Product",
		"ActiveNav": "products",
		"User":      user,
		"IsEdit":    false,
		"Product":   &domain.Product{Status: domain.StatusDraft},
		"Error":     "",
	})
}

// CreateProduct processes new product creation.
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	user := auth.UserFromContext(r.Context())
	price, _ := strconv.ParseInt(r.FormValue("price"), 10, 64)

	input := CreateProductInput{
		Name:             r.FormValue("name"),
		Slug:             r.FormValue("slug"),
		ShortDescription: r.FormValue("short_description"),
		FullDescription:  r.FormValue("full_description"),
		Price:            price,
		ThumbnailURL:     r.FormValue("thumbnail_url"),
		Status:           domain.ProductStatus(r.FormValue("status")),
	}

	prod, err := h.service.CreateProduct(r.Context(), input)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = h.view.Render(w, "admin", "admin/products/form", map[string]any{
			"Title":     "New Product",
			"ActiveNav": "products",
			"User":      user,
			"IsEdit":    false,
			"Product":   &domain.Product{Name: input.Name, Slug: input.Slug, Price: input.Price, Status: input.Status, ShortDescription: input.ShortDescription, FullDescription: input.FullDescription, ThumbnailURL: input.ThumbnailURL},
			"Error":     err.Error(),
		})
		return
	}

	// Redirect to file management for the newly created product
	http.Redirect(w, r, fmt.Sprintf("/admin/products/%d/files", prod.ID), http.StatusSeeOther)
}

// EditProduct renders the edit product form.
func (h *Handler) EditProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	prod, err := h.service.GetProduct(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user := auth.UserFromContext(r.Context())
	_ = h.view.Render(w, "admin", "admin/products/form", map[string]any{
		"Title":     "Edit Product",
		"ActiveNav": "products",
		"User":      user,
		"IsEdit":    true,
		"Product":   prod,
		"Error":     "",
	})
}

// UpdateProduct processes product updates.
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
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
	price, _ := strconv.ParseInt(r.FormValue("price"), 10, 64)

	input := UpdateProductInput{
		Name:             r.FormValue("name"),
		Slug:             r.FormValue("slug"),
		ShortDescription: r.FormValue("short_description"),
		FullDescription:  r.FormValue("full_description"),
		Price:            price,
		ThumbnailURL:     r.FormValue("thumbnail_url"),
		Status:           domain.ProductStatus(r.FormValue("status")),
	}

	_, err = h.service.UpdateProduct(r.Context(), id, input)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = h.view.Render(w, "admin", "admin/products/form", map[string]any{
			"Title":     "Edit Product",
			"ActiveNav": "products",
			"User":      user,
			"IsEdit":    true,
			"Product":   &domain.Product{ID: id, Name: input.Name, Slug: input.Slug, Price: input.Price, Status: input.Status, ShortDescription: input.ShortDescription, FullDescription: input.FullDescription, ThumbnailURL: input.ThumbnailURL},
			"Error":     err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/admin/products", http.StatusSeeOther)
}

// ToggleStatus handles inline HTMX toggling between Draft and Published.
func (h *Handler) ToggleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	prod, err := h.service.TogglePublish(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render HTMX fragment button using Spark Admin badge-table class
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if prod.Status == domain.StatusPublished {
		fmt.Fprintf(w, `<div id="status-badge-%d"><span class="badge-table success" style="cursor: pointer;" hx-post="/admin/products/%d/toggle-status" hx-target="#status-badge-%d" hx-swap="outerHTML" title="Click to set Draft">Published</span></div>`, prod.ID, prod.ID, prod.ID)
	} else {
		fmt.Fprintf(w, `<div id="status-badge-%d"><span class="badge-table pending" style="cursor: pointer;" hx-post="/admin/products/%d/toggle-status" hx-target="#status-badge-%d" hx-swap="outerHTML" title="Click to Publish">Draft</span></div>`, prod.ID, prod.ID, prod.ID)
	}
}

// ShowFiles renders the digital files manager for a specific product.
func (h *Handler) ShowFiles(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	prod, err := h.service.GetProduct(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user := auth.UserFromContext(r.Context())
	_ = h.view.Render(w, "admin", "admin/products/files", map[string]any{
		"Title":     "Manage Files",
		"ActiveNav": "products",
		"User":      user,
		"Product":   prod,
		"Error":     "",
	})
}

// UploadFile handles digital product file upload and storage.
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Parse multipart with 150MB limit
	if err := r.ParseMultipartForm(MaxFileSize); err != nil {
		http.Error(w, "File size too large or invalid multipart data", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File upload missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	version := strings.TrimSpace(r.FormValue("version"))
	if version == "" {
		version = "1.0.0"
	}

	_, err = h.service.UploadFile(r.Context(), productID, header, version)
	if err != nil {
		prod, _ := h.service.GetProduct(r.Context(), productID)
		user := auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusBadRequest)
		_ = h.view.Render(w, "admin", "admin/products/files", map[string]any{
			"Title":     "Manage Files",
			"ActiveNav": "products",
			"User":      user,
			"Product":   prod,
			"Error":     err.Error(),
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/products/%d/files", productID), http.StatusSeeOther)
}

// DeleteFile removes a file attachment.
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	productID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "fileID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = h.service.DeleteFile(r.Context(), fileID)
	http.Redirect(w, r, fmt.Sprintf("/admin/products/%d/files", productID), http.StatusSeeOther)
}

// DeleteProduct deletes an entire product and cleans up stored files.
func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = h.service.DeleteProduct(r.Context(), id)
	http.Redirect(w, r, "/admin/products", http.StatusSeeOther)
}

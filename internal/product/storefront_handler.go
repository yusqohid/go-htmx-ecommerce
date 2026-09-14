package product

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

// StorefrontHandler handles public, customer-facing HTTP requests for the store catalog.
type StorefrontHandler struct {
	service *Service
	view    *view.View
}

// NewStorefrontHandler constructs a new StorefrontHandler.
func NewStorefrontHandler(service *Service, view *view.View) *StorefrontHandler {
	return &StorefrontHandler{
		service: service,
		view:    view,
	}
}

// Home renders the landing homepage with featured products and value propositions.
func (h *StorefrontHandler) Home(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	// Fetch top 6 latest published products for the homepage
	products, total, err := h.service.ListPublishedProducts(r.Context(), "", 1, 6)
	if err != nil {
		products = []domain.Product{}
	}

	_ = h.view.Render(w, "public", "storefront/home", map[string]any{
		"Title":            "Sellora - Curated Digital Products for Developers & Creators",
		"MetaDesc":         "Instant access to curated developer kits, digital assets, and production-ready source code templates.",
		"ActiveNav":        "home",
		"User":             user,
		"FeaturedProducts": products,
		"TotalPublished":   total,
	})
}

// Catalog renders the searchable, paginated catalog of published digital goods.
func (h *StorefrontHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	search := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit := 9
	products, total, err := h.service.ListPublishedProducts(r.Context(), search, page, limit)
	if err != nil {
		products = []domain.Product{}
		total = 0
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	_ = h.view.Render(w, "public", "storefront/catalog", map[string]any{
		"Title":       "Catalog - Explore Digital Assets | Sellora",
		"MetaDesc":    "Discover top-tier digital assets, boilerplates, and developer resources.",
		"ActiveNav":   "catalog",
		"User":        user,
		"Products":    products,
		"Total":       total,
		"Search":      search,
		"CurrentPage": page,
		"TotalPages":  totalPages,
	})
}

// ProductDetail renders the full specification and purchase call-to-action for a product.
func (h *StorefrontHandler) ProductDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	slug := chi.URLParam(r, "slug")

	product, err := h.service.GetPublishedProductBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = h.view.Render(w, "public", "storefront/404", map[string]any{
				"Title":     "Product Not Found - Sellora",
				"ActiveNav": "catalog",
				"User":      user,
			})
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Calculate total digital download size
	var totalSize int64
	for _, f := range product.Files {
		totalSize += f.FileSize
	}

	_ = h.view.Render(w, "public", "storefront/detail", map[string]any{
		"Title":      product.Name + " - Sellora",
		"MetaDesc":   product.ShortDescription,
		"ActiveNav":  "catalog",
		"User":       user,
		"Product":    product,
		"Files":      product.Files,
		"TotalFiles": len(product.Files),
		"TotalSize":  totalSize,
	})
}

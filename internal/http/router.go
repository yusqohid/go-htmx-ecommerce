package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
	"github.com/yusqohid/go-htmx-ecommerce/internal/customer"
	"github.com/yusqohid/go-htmx-ecommerce/internal/database"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/order"
	"github.com/yusqohid/go-htmx-ecommerce/internal/payment"
	"github.com/yusqohid/go-htmx-ecommerce/internal/product"
	"github.com/yusqohid/go-htmx-ecommerce/web/static"
)

// RouterDeps encapsulates all dependencies needed to configure the HTTP router.
type RouterDeps struct {
	Config            *config.Config
	DB                *database.DB
	AuthService       *auth.Service
	AuthHandler       *auth.Handler
	ProductHandler    *product.Handler
	StorefrontHandler *product.StorefrontHandler
	OrderHandler      *order.Handler
	PaymentHandler    *payment.Handler
	DownloadHandler   *product.DownloadHandler
	CustomerHandler   *customer.Handler
	AdminOrderHandler *order.AdminHandler
}

// NewRouter sets up the Chi HTTP router with base middlewares, static files, auth, and routes.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	isProduction := false
	if deps.Config != nil {
		isProduction = deps.Config.IsProduction()
	}

	// Base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(SecurityHeaders(isProduction))
	r.Use(CSRF(isProduction))
	r.Use(middleware.Timeout(60 * time.Second))
	// Attach user session to request context if present (must be before any routes)
	if deps.AuthService != nil {
		r.Use(auth.Authenticate(deps.AuthService))
	}

	// Serve static assets (CSS, JS, Fonts, Images from Spark Admin)
	r.Handle("/static/*", static.Handler())

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "connected"
		isHealthy := true
		if deps.DB != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := deps.DB.PingContext(ctx); err != nil {
				dbStatus = "unreachable"
				isHealthy = false
			}
		} else {
			dbStatus = "not configured"
			isHealthy = false
		}

		status := "ok"
		w.Header().Set("Content-Type", "application/json")
		if !isHealthy {
			status = "degraded"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   status,
			"app":      "Sellora",
			"env":      deps.Config.AppEnv,
			"database": dbStatus,
			"time":     time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Public / Guest authentication routes
	if deps.AuthHandler != nil {
		r.Group(func(guest chi.Router) {
			guest.Use(auth.RequireGuest("/"))
			guest.Get("/login", deps.AuthHandler.ShowLogin)
			guest.Post("/login", deps.AuthHandler.Login)
			guest.Get("/register", deps.AuthHandler.ShowRegister)
			guest.Post("/register", deps.AuthHandler.Register)
		})

		r.Post("/logout", deps.AuthHandler.Logout)
		r.Get("/logout", deps.AuthHandler.Logout)
	}

	// Public Storefront routes
	if deps.StorefrontHandler != nil {
		r.Get("/", deps.StorefrontHandler.Home)
		r.Get("/products", deps.StorefrontHandler.Catalog)
		r.Get("/products/{slug}", deps.StorefrontHandler.ProductDetail)
	}

	// Order & Checkout routes
	if deps.OrderHandler != nil {
		// Checkout flow requires customer authentication
		r.Group(func(authRouter chi.Router) {
			authRouter.Use(auth.RequireAuth("/login"))
			authRouter.Get("/checkout/{productID}", deps.OrderHandler.CheckoutPage)
			authRouter.Post("/checkout/{productID}", deps.OrderHandler.ProcessCheckout)
		})

		r.Get("/orders/{reference}/success", deps.OrderHandler.OrderSuccess)

		// Sandbox Mock Gateway Routes (accessible in development only when mock provider enabled)
		if deps.Config != nil && !deps.Config.IsProduction() && deps.Config.PaymentProvider == "mock" {
			r.Get("/mock-checkout", deps.OrderHandler.MockCheckoutPage)
			r.Post("/mock-checkout/simulate", deps.OrderHandler.MockSimulatePayment)
		}
	}

	// Payment Provider Webhooks (LYNK.ID, Mock, etc.)
	if deps.PaymentHandler != nil {
		r.Post("/webhooks/{provider}", deps.PaymentHandler.HandleWebhook)
	}

	// Secure Digital Product Downloads (requires customer authentication & paid order)
	if deps.DownloadHandler != nil {
		r.Get("/downloads/{fileID}", deps.DownloadHandler.DownloadFile)
	}

	// Customer Account & Order History
	if deps.CustomerHandler != nil {
		r.Group(func(cust chi.Router) {
			cust.Use(auth.RequireAuth("/login"))
			cust.Get("/account", deps.CustomerHandler.AccountHome)
			cust.Get("/account/orders", deps.CustomerHandler.ListOrders)
			cust.Get("/account/orders/{id}", deps.CustomerHandler.OrderDetail)
		})
	}

	// Protected Admin Dashboard & Product Management
	if deps.ProductHandler != nil {
		r.Group(func(admin chi.Router) {
			admin.Use(auth.RequireAuth("/login"))
			admin.Use(auth.RequireRole(domain.RoleAdmin))

			admin.Get("/admin", deps.ProductHandler.Dashboard)
			admin.Get("/admin/products", deps.ProductHandler.ListProducts)
			admin.Get("/admin/products/new", deps.ProductHandler.NewProduct)
			admin.Post("/admin/products", deps.ProductHandler.CreateProduct)
			admin.Get("/admin/products/{id}/edit", deps.ProductHandler.EditProduct)
			admin.Post("/admin/products/{id}", deps.ProductHandler.UpdateProduct)
			admin.Post("/admin/products/{id}/toggle-status", deps.ProductHandler.ToggleStatus)
			admin.Get("/admin/products/{id}/files", deps.ProductHandler.ShowFiles)
			admin.Post("/admin/products/{id}/files", deps.ProductHandler.UploadFile)
			admin.Post("/admin/products/{id}/files/{fileID}/delete", deps.ProductHandler.DeleteFile)
			admin.Post("/admin/products/{id}/delete", deps.ProductHandler.DeleteProduct)

			// Admin Orders Management
			if deps.AdminOrderHandler != nil {
				admin.Get("/admin/orders", deps.AdminOrderHandler.ListOrders)
				admin.Get("/admin/orders/{id}", deps.AdminOrderHandler.OrderDetail)
			}
		})
	}

	return r
}

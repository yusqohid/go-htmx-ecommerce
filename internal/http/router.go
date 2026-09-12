package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
	"github.com/yusqohid/go-htmx-ecommerce/internal/database"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/product"
	"github.com/yusqohid/go-htmx-ecommerce/web/static"
)

// RouterDeps encapsulates all dependencies needed to configure the HTTP router.
type RouterDeps struct {
	Config         *config.Config
	DB             *database.DB
	AuthService    *auth.Service
	AuthHandler    *auth.Handler
	ProductHandler *product.Handler
}

// NewRouter sets up the Chi HTTP router with base middlewares, static files, auth, and routes.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
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
		if deps.DB != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := deps.DB.PingContext(ctx); err != nil {
				dbStatus = "unreachable"
			}
		} else {
			dbStatus = "not configured"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "ok",
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

	// Placeholder Homepage (Fase 4 will provide full storefront)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r.Context())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if user != nil {
			adminLink := ""
			if user.IsAdmin() {
				adminLink = `<p><a href="/admin" style="font-weight:bold;color:#16a34a;">Go to Admin Dashboard &rarr;</a></p>`
			}
			_, _ = fmt.Fprintf(w, `
				<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem;">
				<h2>Welcome back, %s (%s)!</h2>
				<p>Role: <strong>%s</strong></p>
				%s
				<p><a href="/logout">Logout</a></p>
				</body></html>
			`, user.Name, user.Email, user.Role, adminLink)
			return
		}
		_, _ = fmt.Fprintf(w, `
			<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem;">
			<h2>Welcome to Sellora</h2>
			<p><a href="/login">Sign In</a> | <a href="/register">Register</a></p>
			</body></html>
		`)
	})

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

			// Placeholder orders route
			admin.Get("/admin/orders", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, `<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem;"><h2>Orders Management</h2><p>Coming in Phase 5 & 8!</p><p><a href="/admin">&larr; Back to Dashboard</a></p></body></html>`)
			})
		})
	}

	return r
}

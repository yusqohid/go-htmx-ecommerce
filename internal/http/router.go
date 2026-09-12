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
)

// RouterDeps encapsulates all dependencies needed to configure the HTTP router.
type RouterDeps struct {
	Config      *config.Config
	DB          *database.DB
	AuthService *auth.Service
	AuthHandler *auth.Handler
}

// NewRouter sets up the Chi HTTP router with base middlewares, authentication, and routes.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Attach user session to request context if present
	if deps.AuthService != nil {
		r.Use(auth.Authenticate(deps.AuthService))
	}

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

	// Placeholder Homepage
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r.Context())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if user != nil {
			_, _ = fmt.Fprintf(w, `
				<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem;">
				<h2>Welcome back, %s (%s)!</h2>
				<p>Role: <strong>%s</strong></p>
				<p><a href="/logout">Logout</a></p>
				</body></html>
			`, user.Name, user.Email, user.Role)
			return
		}
		_, _ = fmt.Fprintf(w, `
			<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem;">
			<h2>Welcome to Sellora</h2>
			<p><a href="/login">Sign In</a> | <a href="/register">Register</a></p>
			</body></html>
		`)
	})

	// Admin protected area (Fase 2 demo / validation)
	r.Group(func(admin chi.Router) {
		admin.Use(auth.RequireAuth("/login"))
		admin.Use(auth.RequireRole(domain.RoleAdmin))
		admin.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
			user := auth.UserFromContext(r.Context())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `
				<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem;">
				<h1>Sellora Admin Dashboard</h1>
				<p>Authenticated as Administrator: <strong>%s</strong> (%s)</p>
				<p><a href="/logout">Logout</a></p>
				</body></html>
			`, user.Name, user.Email)
		})
	})

	return r
}

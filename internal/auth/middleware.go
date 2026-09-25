package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

type contextKey string

const (
	SessionCookieName            = "sellora_session"
	userContextKey    contextKey = "sellora_auth_user"
	tokenContextKey   contextKey = "sellora_auth_token"
)

// UserFromContext retrieves the authenticated user from the request context.
func UserFromContext(ctx context.Context) *domain.User {
	u, ok := ctx.Value(userContextKey).(*domain.User)
	if !ok {
		return nil
	}
	return u
}

// WithUser returns a copy of parent context with the authenticated user attached.
func WithUser(parent context.Context, user *domain.User) context.Context {
	return context.WithValue(parent, userContextKey, user)
}

// TokenFromContext retrieves the active session token from the request context.
func TokenFromContext(ctx context.Context) string {
	t, ok := ctx.Value(tokenContextKey).(string)
	if !ok {
		return ""
	}
	return t
}

// Authenticate is a middleware that inspects the session cookie and attaches the user to the context.
func Authenticate(service *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			user, err := service.AuthenticateToken(r.Context(), cookie.Value)
			if err != nil || user == nil {
				// Clear invalid or expired cookie
				ClearSessionCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			ctx = context.WithValue(ctx, tokenContextKey, cookie.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth ensures that the request is made by an authenticated user.
func RequireAuth(defaultRedirect string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				if r.Header.Get("HX-Request") == "true" {
					w.Header().Set("HX-Redirect", "/login")
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				target := "/login"
				if r.URL.Path != "" && r.URL.Path != "/" {
					target = "/login?redirect=" + url.QueryEscape(r.URL.RequestURI())
				}
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole ensures that the user has the specified authorization role.
func RequireRole(role domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			if user.Role != role {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireGuest redirects already authenticated users away from guest-only pages (e.g. login, register).
func RequireGuest(defaultRedirect string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user != nil {
				if user.IsAdmin() {
					http.Redirect(w, r, "/admin", http.StatusSeeOther)
					return
				}
				http.Redirect(w, r, defaultRedirect, http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SetSessionCookie sets a secure, HTTP-only session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, isProduction bool) {
	secure := isProduction && (r.TLS != nil || strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https")
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(SessionDuration.Seconds()),
	})
}

// ClearSessionCookie invalidates the session cookie on the client.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

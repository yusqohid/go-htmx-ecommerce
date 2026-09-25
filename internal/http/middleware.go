package http

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

type contextKey string

const csrfContextKey contextKey = "sellora_csrf_token"
const csrfCookieName = "sellora_csrf"

// SecurityHeaders returns a middleware that attaches defensive HTTP headers to every response.
func SecurityHeaders(isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Clickjacking mitigation: disallow framing
			w.Header().Set("X-Frame-Options", "DENY")

			// MIME-sniffing mitigation: enforce declared content type
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Referrer policy: send origin only on cross-origin requests
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// XSS auditor configuration for legacy user agents
			w.Header().Set("X-XSS-Protection", "0")

			// Enforce HTTPS in production with Strict-Transport-Security (1 year)
			if isProduction {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CSRFTokenFromContext retrieves the active CSRF token from the request context.
func CSRFTokenFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(csrfContextKey).(string); ok {
		return val
	}
	return ""
}

// CSRF returns a middleware implementing double-submit cookie CSRF protection,
// compatible with both standard HTML forms and HTMX asynchronous requests.
func CSRF(isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF validation for external payment provider webhooks
			// (webhooks are cryptographically authenticated via HMAC/SHA512 provider signatures).
			if strings.HasPrefix(r.URL.Path, "/webhooks/") {
				next.ServeHTTP(w, r)
				return
			}

			// 1. Retrieve or generate cookie token
			var token string
			cookie, err := r.Cookie(csrfCookieName)
			if err == nil && len(cookie.Value) == 64 {
				token = cookie.Value
			} else {
				// Generate 32-byte (64 hex characters) cryptographically secure random token
				b := make([]byte, 32)
				if _, err := rand.Read(b); err == nil {
					token = hex.EncodeToString(b)
					http.SetCookie(w, &http.Cookie{
						Name:     csrfCookieName,
						Value:    token,
						Path:     "/",
						HttpOnly: false, // Accessible to client-side script for HTMX / form injection
						Secure:   isProduction,
						SameSite: http.SameSiteLaxMode,
					})
				}
			}

			// Attach token to request context
			ctx := context.WithValue(r.Context(), csrfContextKey, token)
			r = r.WithContext(ctx)

			// 2. Safe HTTP methods require no token verification
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				next.ServeHTTP(w, r)
				return
			}

			// 3. State-changing methods (POST, PUT, PATCH, DELETE) require CSRF token validation
			submittedToken := r.Header.Get("X-CSRF-Token")
			if submittedToken == "" {
				submittedToken = r.Header.Get("HX-CSRFToken")
			}
			if submittedToken == "" {
				// Check form value (ParseForm parses URL and body query parameters)
				_ = r.ParseForm()
				submittedToken = r.FormValue("csrf_token")
				if submittedToken == "" {
					submittedToken = r.FormValue("_csrf")
				}
			}

			if token == "" || submittedToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(submittedToken)) != 1 {
				http.Error(w, "Forbidden: Invalid or missing CSRF token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

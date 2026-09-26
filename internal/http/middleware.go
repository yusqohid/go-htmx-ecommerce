package http

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type contextKey string

const csrfContextKey contextKey = "sellora_csrf_token"
const csrfCookieName = "sellora_csrf"

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https" {
		return true
	}
	return false
}

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

			// Enforce HTTPS in production when connection is secure
			if isProduction && isSecureRequest(r) {
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
						Secure:   isProduction && isSecureRequest(r),
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
				contentType := r.Header.Get("Content-Type")
				if strings.HasPrefix(contentType, "multipart/form-data") {
					_ = r.ParseMultipartForm(32 << 20)
				} else {
					_ = r.ParseForm()
				}
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
// StructuredLogger produces structured, machine-readable HTTP request logs via log/slog.
func StructuredLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				duration := time.Since(start)
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}

				reqID := middleware.GetReqID(r.Context())

				attrs := []slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Float64("duration_ms", float64(duration.Microseconds())/1000.0),
					slog.String("remote_ip", r.RemoteAddr),
				}
				if reqID != "" {
					attrs = append(attrs, slog.String("request_id", reqID))
				}

				level := slog.LevelInfo
				if status >= 500 {
					level = slog.LevelError
				} else if status >= 400 {
					level = slog.LevelWarn
				}

				logger.LogAttrs(r.Context(), level, "HTTP request completed", attrs...)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// PanicRecovery gracefully catches runtime panics, logs the stack trace with slog,
// and returns a safe 500 Internal Server Error without leaking internal stack traces.
func PanicRecovery(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					if rvr == http.ErrAbortHandler {
						panic(rvr)
					}

					reqID := middleware.GetReqID(r.Context())
					stack := string(debug.Stack())

					logger.ErrorContext(r.Context(), "Unhandled panic recovered in HTTP handler",
						slog.Any("error", rvr),
						slog.String("stack", stack),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("request_id", reqID),
					)

					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>500 Internal Server Error</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
  <h1>500 - Internal Server Error</h1>
  <p>Something went wrong on our end. Please try again later.</p>
</body>
</html>`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

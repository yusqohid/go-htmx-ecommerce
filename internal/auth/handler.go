package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

// Handler handles HTTP requests for user authentication.
type Handler struct {
	authService  *Service
	view         *view.View
	isProduction bool
}

// NewHandler creates a new auth Handler.
func NewHandler(authService *Service, view *view.View, isProduction bool) *Handler {
	return &Handler{
		authService:  authService,
		view:         view,
		isProduction: isProduction,
	}
}

// ShowLogin renders the sign-in page.
func (h *Handler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	redirect := r.URL.Query().Get("redirect")
	_ = h.view.Render(w, "auth", "auth/login", map[string]any{
		"Title":    "Sign In",
		"Redirect": redirect,
		"Email":    "",
		"Error":    "",
	})
}

// Login processes credentials, sets a session cookie, and redirects the user.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	redirect := r.FormValue("redirect")

	user, session, err := h.authService.Login(r.Context(), email, password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = h.view.Render(w, "auth", "auth/login", map[string]any{
			"Title":    "Sign In",
			"Redirect": redirect,
			"Email":    email,
			"Error":    "Invalid email or password",
		})
		return
	}

	SetSessionCookie(w, session.Token, h.isProduction)

	// Determine redirection target
	if user.IsAdmin() {
		if strings.HasPrefix(redirect, "/admin") {
			http.Redirect(w, r, redirect, http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	if redirect != "" && strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "/login") && !strings.HasPrefix(redirect, "/admin") {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ShowRegister renders the customer registration page.
func (h *Handler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	_ = h.view.Render(w, "auth", "auth/register", map[string]any{
		"Title": "Create Account",
		"Name":  "",
		"Email": "",
		"Error": "",
	})
}

// Register processes customer registration and logs the user in.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")

	renderError := func(errMsg string, status int) {
		w.WriteHeader(status)
		_ = h.view.Render(w, "auth", "auth/register", map[string]any{
			"Title": "Create Account",
			"Name":  name,
			"Email": email,
			"Error": errMsg,
		})
	}

	if password != confirm {
		renderError("Passwords do not match", http.StatusBadRequest)
		return
	}

	_, session, err := h.authService.RegisterCustomer(r.Context(), name, email, password)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			renderError("An account with this email address already exists", http.StatusConflict)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			renderError(err.Error(), http.StatusBadRequest)
			return
		}
		renderError("Failed to create account. Please try again.", http.StatusInternalServerError)
		return
	}

	SetSessionCookie(w, session.Token, h.isProduction)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout terminates the current session and clears the cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := TokenFromContext(r.Context())
	if token == "" {
		if cookie, err := r.Cookie(SessionCookieName); err == nil {
			token = cookie.Value
		}
	}

	_ = h.authService.Logout(r.Context(), token)
	ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

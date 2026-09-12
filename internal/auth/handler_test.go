package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
	"github.com/yusqohid/go-htmx-ecommerce/web/templates"
)

func setupTestHandler() (*auth.Handler, *auth.Service, *MockUserRepository) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	viewRenderer := view.New(templates.FS, true)
	handler := auth.NewHandler(service, viewRenderer, false)
	return handler, service, userRepo
}

func TestShowLogin(t *testing.T) {
	handler, _, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()

	handler.ShowLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Sign in to Sellora") {
		t.Errorf("expected body to contain 'Sign in to Sellora'")
	}
}

func TestShowRegister(t *testing.T) {
	handler, _, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rr := httptest.NewRecorder()

	handler.ShowRegister(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Create your account") {
		t.Errorf("expected body to contain 'Create your account'")
	}
}

func TestHandlerRegisterAndLogin(t *testing.T) {
	handler, service, _ := setupTestHandler()

	// 1. Password mismatch error
	formMismatch := url.Values{
		"name":             {"John"},
		"email":            {"john@example.com"},
		"password":         {"Password123"},
		"password_confirm": {"MismatchPass"},
	}
	reqMismatch := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(formMismatch.Encode()))
	reqMismatch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrMismatch := httptest.NewRecorder()
	handler.Register(rrMismatch, reqMismatch)

	if rrMismatch.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for password mismatch, got %d", rrMismatch.Code)
	}

	// 2. Successful customer registration
	formValid := url.Values{
		"name":             {"John Doe"},
		"email":            {"john@example.com"},
		"password":         {"Password123"},
		"password_confirm": {"Password123"},
	}
	reqValid := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(formValid.Encode()))
	reqValid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrValid := httptest.NewRecorder()
	handler.Register(rrValid, reqValid)

	if rrValid.Code != http.StatusSeeOther {
		t.Errorf("expected 303 See Other redirect after register, got %d", rrValid.Code)
	}

	cookies := rrValid.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Errorf("expected session cookie to be set after registration")
	}

	// 3. Login with wrong password
	formWrong := url.Values{
		"email":    {"john@example.com"},
		"password": {"WrongSecret"},
	}
	reqWrong := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formWrong.Encode()))
	reqWrong.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrWrong := httptest.NewRecorder()
	handler.Login(rrWrong, reqWrong)

	if rrWrong.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for wrong credentials, got %d", rrWrong.Code)
	}

	// 4. Successful Admin Login redirects to /admin
	_, _ = service.CreateAdmin(context.Background(), "Super Admin", "admin@store.com", "AdminPassword123")
	formAdmin := url.Values{
		"email":    {"admin@store.com"},
		"password": {"AdminPassword123"},
	}
	reqAdmin := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formAdmin.Encode()))
	reqAdmin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrAdmin := httptest.NewRecorder()
	handler.Login(rrAdmin, reqAdmin)

	if rrAdmin.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for admin login, got %d", rrAdmin.Code)
	}
	if rrAdmin.Header().Get("Location") != "/admin" {
		t.Errorf("expected admin to be redirected to /admin, got %s", rrAdmin.Header().Get("Location"))
	}

	// 5. Logout
	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rrLogout := httptest.NewRecorder()
	handler.Logout(rrLogout, reqLogout)

	if rrLogout.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect after logout, got %d", rrLogout.Code)
	}
	logoutCookies := rrLogout.Result().Cookies()
	var clearedCookie *http.Cookie
	for _, c := range logoutCookies {
		if c.Name == auth.SessionCookieName {
			clearedCookie = c
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge != -1 {
		t.Errorf("expected session cookie to be cleared with MaxAge -1")
	}
}

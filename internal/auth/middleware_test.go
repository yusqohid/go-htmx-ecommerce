package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

func TestRequireAuthMiddleware(t *testing.T) {
	middleware := auth.RequireAuth("/login")

	// Protected dummy handler
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("protected content"))
	})

	// 1. Unauthenticated request should be redirected
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rr := httptest.NewRecorder()
	middleware(dummyHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 See Other redirect, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/login?redirect=%2Fdashboard" {
		t.Errorf("unexpected redirect location: %s", location)
	}

	// 2. Authenticated request should pass through
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)

	user, session, err := service.RegisterCustomer(context.Background(), "Dave", "dave@example.com", "Password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	authMiddleware := auth.Authenticate(service)
	reqAuth := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	reqAuth.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: session.Token,
	})

	rrAuth := httptest.NewRecorder()
	// Chain: Authenticate -> RequireAuth -> dummyHandler
	authMiddleware(middleware(dummyHandler)).ServeHTTP(rrAuth, reqAuth)

	if rrAuth.Code != http.StatusOK {
		t.Errorf("expected 200 OK for authenticated user, got %d", rrAuth.Code)
	}
	_ = user
}

func TestRequireRoleMiddleware(t *testing.T) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	ctx := context.Background()

	// Register Customer
	_, customerSession, _ := service.RegisterCustomer(ctx, "Customer", "cust@example.com", "Password123")

	// Create Admin
	_, _ = service.CreateAdmin(ctx, "Admin", "admin@store.com", "Password123")
	_, adminSession, _ := service.Login(ctx, "admin@store.com", "Password123")

	adminOnlyMiddleware := auth.RequireRole(domain.RoleAdmin)
	dummyAdminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("admin only area"))
	})

	authMiddleware := auth.Authenticate(service)

	// 1. Customer accessing Admin route should get 403 Forbidden
	reqCust := httptest.NewRequest(http.MethodGet, "/admin", nil)
	reqCust.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: customerSession.Token})
	rrCust := httptest.NewRecorder()
	authMiddleware(adminOnlyMiddleware(dummyAdminHandler)).ServeHTTP(rrCust, reqCust)

	if rrCust.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for customer accessing admin route, got %d", rrCust.Code)
	}

	// 2. Admin accessing Admin route should succeed with 200 OK
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: adminSession.Token})
	rrAdmin := httptest.NewRecorder()
	authMiddleware(adminOnlyMiddleware(dummyAdminHandler)).ServeHTTP(rrAdmin, reqAdmin)

	if rrAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for admin accessing admin route, got %d", rrAdmin.Code)
	}
}

func TestRequireGuestMiddleware(t *testing.T) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	ctx := context.Background()

	_, session, _ := service.RegisterCustomer(ctx, "GuestTester", "guest@example.com", "Password123")

	guestMiddleware := auth.RequireGuest("/")
	dummyLoginHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("login form"))
	})

	authMiddleware := auth.Authenticate(service)

	// 1. Authenticated user accessing /login should be redirected away
	reqAuth := httptest.NewRequest(http.MethodGet, "/login", nil)
	reqAuth.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	rrAuth := httptest.NewRecorder()
	authMiddleware(guestMiddleware(dummyLoginHandler)).ServeHTTP(rrAuth, reqAuth)

	if rrAuth.Code != http.StatusSeeOther {
		t.Errorf("expected 303 See Other redirect for logged in user on guest route, got %d", rrAuth.Code)
	}

	// 2. Unauthenticated user should see login form
	reqGuest := httptest.NewRequest(http.MethodGet, "/login", nil)
	rrGuest := httptest.NewRecorder()
	authMiddleware(guestMiddleware(dummyLoginHandler)).ServeHTTP(rrGuest, reqGuest)

	if rrGuest.Code != http.StatusOK {
		t.Errorf("expected 200 OK for unauthenticated guest, got %d", rrGuest.Code)
	}
}

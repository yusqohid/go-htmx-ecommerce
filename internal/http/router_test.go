package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/config"
	appHTTP "github.com/yusqohid/go-htmx-ecommerce/internal/http"
)

func TestHealthcheckEndpoint(t *testing.T) {
	cfg := &config.Config{
		AppEnv:  "testing",
		AppPort: "8080",
	}

	router := appHTTP.NewRouter(appHTTP.RouterDeps{
		Config: cfg,
		DB:     nil,
	})

	req, err := http.NewRequest(http.MethodGet, "/healthz", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 Service Unavailable when DB is nil, got %d", rr.Code)
	}

	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response json: %v", err)
	}

	if response["status"] != "degraded" {
		t.Errorf("expected status 'degraded', got %v", response["status"])
	}
	if response["app"] != "Sellora" {
		t.Errorf("expected app 'Sellora', got %v", response["app"])
	}
}
func TestSecurityHeaders(t *testing.T) {
	cfg := &config.Config{
		AppEnv:  "production",
		AppPort: "8080",
	}

	router := appHTTP.NewRouter(appHTTP.RouterDeps{
		Config: cfg,
	})

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected X-Frame-Options DENY, got %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options nosniff, got %q", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy strict-origin-when-cross-origin, got %q", got)
	}
	if got := rr.Header().Get("Strict-Transport-Security"); got == "" {
		t.Errorf("expected Strict-Transport-Security in production, got empty")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	cfg := &config.Config{
		AppEnv:  "development",
		AppPort: "8080",
	}

	router := appHTTP.NewRouter(appHTTP.RouterDeps{
		Config: cfg,
	})

	// 1. Initial GET request sets CSRF cookie
	getReq, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)

	var csrfCookie *http.Cookie
	for _, c := range getRec.Result().Cookies() {
		if c.Name == "sellora_csrf" {
			csrfCookie = c
			break
		}
	}

	if csrfCookie == nil || len(csrfCookie.Value) != 64 {
		t.Fatalf("expected valid 64-char sellora_csrf cookie, got %v", csrfCookie)
	}

	// 2. POST without CSRF token must fail with 403 Forbidden
	postReq, _ := http.NewRequest(http.MethodPost, "/login", nil)
	postReq.AddCookie(csrfCookie)
	postRec := httptest.NewRecorder()
	router.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusForbidden {
		t.Errorf("expected POST without token to fail 403 Forbidden, got %d", postRec.Code)
	}

	// 3. POST with valid header token passes CSRF
	postReqWithHeader, _ := http.NewRequest(http.MethodPost, "/login", nil)
	postReqWithHeader.AddCookie(csrfCookie)
	postReqWithHeader.Header.Set("X-CSRF-Token", csrfCookie.Value)
	postRecWithHeader := httptest.NewRecorder()
	router.ServeHTTP(postRecWithHeader, postReqWithHeader)

	// Since AuthHandler is nil in this test, router returns 404 once past CSRF
	if postRecWithHeader.Code == http.StatusForbidden {
		t.Errorf("expected POST with valid token to pass CSRF, but got 403")
	}
}

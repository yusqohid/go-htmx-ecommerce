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

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response json: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", response["status"])
	}

	if response["app"] != "Sellora" {
		t.Errorf("expected app 'Sellora', got %v", response["app"])
	}
}

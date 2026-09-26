package http_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appHTTP "github.com/yusqohid/go-htmx-ecommerce/internal/http"
)

func TestStructuredLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := appHTTP.StructuredLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok response"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"method":"GET"`) {
		t.Errorf("expected log to contain GET method, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"path":"/test-path"`) {
		t.Errorf("expected log to contain path, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"status":200`) {
		t.Errorf("expected log to contain status 200, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"bytes":11`) {
		t.Errorf("expected log to contain bytes 11, got: %s", logOutput)
	}
}

func TestPanicRecovery(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	panickingHandler := appHTTP.PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated unexpected panic in handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic-route", nil)
	rec := httptest.NewRecorder()

	panickingHandler.ServeHTTP(rec, req)

	// Verify status is 500 Internal Server Error
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	// Verify response body does not expose internal stack trace
	body := rec.Body.String()
	if !strings.Contains(body, "500 - Internal Server Error") {
		t.Errorf("expected friendly 500 error page, got: %s", body)
	}
	if strings.Contains(body, "simulated unexpected panic") {
		t.Errorf("response body should not leak raw panic string")
	}

	// Verify logger recorded the panic and stack trace
	logOutput := buf.String()
	if !strings.Contains(logOutput, "Unhandled panic recovered in HTTP handler") {
		t.Errorf("expected log to contain panic recovery message, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "simulated unexpected panic in handler") {
		t.Errorf("expected log to contain raw panic error, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"stack":`) {
		t.Errorf("expected log to contain stack trace attribute, got: %s", logOutput)
	}
}

func TestPanicRecovery_AbortHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	abortHandler := appHTTP.PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	rec := httptest.NewRecorder()

	defer func() {
		rvr := recover()
		if rvr != http.ErrAbortHandler {
			t.Errorf("expected http.ErrAbortHandler to be re-panicked, got %v", rvr)
		}
	}()

	abortHandler.ServeHTTP(rec, req)
}

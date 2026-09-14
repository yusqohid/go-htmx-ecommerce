package view_test

import (
	"html/template"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
)

func TestFuncMap(t *testing.T) {
	funcs := view.FuncMap()

	// formatMoney test
	formatMoney := funcs["formatMoney"].(func(int64) string)
	if got := formatMoney(150000); got != "Rp 150.000" {
		t.Errorf("formatMoney(150000) = %q, want 'Rp 150.000'", got)
	}

	// formatBytes test
	formatBytes := funcs["formatBytes"].(func(int64) string)
	if got := formatBytes(500); got != "500 B" {
		t.Errorf("formatBytes(500) = %q, want '500 B'", got)
	}
	if got := formatBytes(1024 * 1024 * 5); got != "5.0 MB" {
		t.Errorf("formatBytes(5MB) = %q, want '5.0 MB'", got)
	}

	// formatDateShort test
	formatDateShort := funcs["formatDateShort"].(func(time.Time) string)
	testTime := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	if got := formatDateShort(testTime); got != "14 Sep 2026" {
		t.Errorf("formatDateShort() = %q, want '14 Sep 2026'", got)
	}

	// nl2br test
	nl2br := funcs["nl2br"].(func(string) template.HTML)
	if got := nl2br("line1\nline2"); got != "line1<br>line2" {
		t.Errorf("nl2br() = %q, want 'line1<br>line2'", got)
	}

	// add and sub test
	add := funcs["add"].(func(int, int) int)
	if got := add(2, 3); got != 5 {
		t.Errorf("add(2, 3) = %d, want 5", got)
	}
	sub := funcs["sub"].(func(int, int) int)
	if got := sub(5, 2); got != 3 {
		t.Errorf("sub(5, 2) = %d, want 3", got)
	}
}

func TestViewRender(t *testing.T) {
	mockFS := fstest.MapFS{
		"layouts/main.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{ template "content" . }}</body></html>`),
		},
		"pages/home.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>Hello, {{ .Name }}</h1>{{ end }}`),
		},
	}

	v := view.New(mockFS, false)
	rec := httptest.NewRecorder()
	err := v.Render(rec, "main", "home", map[string]string{"Name": "Sellora"})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	if rec.Code != 200 {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	expectedBody := `<!DOCTYPE html><html><body><h1>Hello, Sellora</h1></body></html>`
	if rec.Body.String() != expectedBody {
		t.Errorf("body = %q, want %q", rec.Body.String(), expectedBody)
	}
}

package product_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/product"
	"github.com/yusqohid/go-htmx-ecommerce/internal/storage"
)

type mockOwnershipChecker struct {
	allowed map[string]bool
}

func (m *mockOwnershipChecker) HasAccessToProduct(ctx context.Context, userID, productID int64) (bool, error) {
	key := string(rune(userID)) + ":" + string(rune(productID))
	return m.allowed[key], nil
}

func TestDownloadHandler(t *testing.T) {
	tempDir := t.TempDir()
	storageMgr, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Save test file to storage
	fileContent := []byte("SECRET-EBOOK-DATA-12345")
	storageKey, size, checksum, err := storageMgr.Save(bytes.NewReader(fileContent), "my-ebook.pdf")
	if err != nil {
		t.Fatalf("failed to save file to storage: %v", err)
	}

	fileRepo := NewMockProductFileRepository()
	_ = fileRepo.Create(context.Background(), &domain.ProductFile{
		ID:           1,
		ProductID:    100,
		OriginalName: "my-ebook.pdf",
		StoragePath:  storageKey,
		FileSize:     size,
		MIMEType:     "application/pdf",
		Checksum:     checksum,
	})

	ownership := &mockOwnershipChecker{
		allowed: map[string]bool{
			string(rune(10)) + ":" + string(rune(100)): true, // user 10 purchased product 100
		},
	}

	handler := product.NewDownloadHandler(fileRepo, ownership, storageMgr)

	r := chi.NewRouter()
	r.Get("/downloads/{fileID}", handler.DownloadFile)

	// 1. Unauthenticated -> redirect to /login
	reqUnauth := httptest.NewRequest(http.MethodGet, "/downloads/1", nil)
	recUnauth := httptest.NewRecorder()
	r.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect to /login, got %d", recUnauth.Code)
	}

	// 2. Authenticated user who hasn't purchased -> 403 Forbidden
	unauthorizedUser := &domain.User{ID: 99, Role: domain.RoleCustomer}
	reqForbidden := httptest.NewRequest(http.MethodGet, "/downloads/1", nil)
	reqForbidden = reqForbidden.WithContext(auth.WithUser(reqForbidden.Context(), unauthorizedUser))
	recForbidden := httptest.NewRecorder()
	r.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", recForbidden.Code)
	}

	// 3. Authenticated customer who purchased -> 200 OK + file stream
	paidCustomer := &domain.User{ID: 10, Role: domain.RoleCustomer}
	reqPaid := httptest.NewRequest(http.MethodGet, "/downloads/1", nil)
	reqPaid = reqPaid.WithContext(auth.WithUser(reqPaid.Context(), paidCustomer))
	recPaid := httptest.NewRecorder()
	r.ServeHTTP(recPaid, reqPaid)
	if recPaid.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recPaid.Code)
	}
	if recPaid.Body.String() != string(fileContent) {
		t.Errorf("got body %q, want %q", recPaid.Body.String(), string(fileContent))
	}
	if recPaid.Header().Get("Content-Disposition") != `attachment; filename="my-ebook.pdf"` {
		t.Errorf("unexpected Content-Disposition: %s", recPaid.Header().Get("Content-Disposition"))
	}

	// 4. Admin user -> 200 OK (even without purchase)
	adminUser := &domain.User{ID: 1, Role: domain.RoleAdmin}
	reqAdmin := httptest.NewRequest(http.MethodGet, "/downloads/1", nil)
	reqAdmin = reqAdmin.WithContext(auth.WithUser(reqAdmin.Context(), adminUser))
	recAdmin := httptest.NewRecorder()
	r.ServeHTTP(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusOK {
		t.Errorf("admin: expected 200 OK, got %d", recAdmin.Code)
	}

	// 5. Non-existent file ID -> 404 Not Found
	reqNotFound := httptest.NewRequest(http.MethodGet, "/downloads/999", nil)
	reqNotFound = reqNotFound.WithContext(auth.WithUser(reqNotFound.Context(), paidCustomer))
	recNotFound := httptest.NewRecorder()
	r.ServeHTTP(recNotFound, reqNotFound)
	if recNotFound.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", recNotFound.Code)
	}
}

package product

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/storage"
)

// OwnershipChecker verifies whether a user has purchased access to a digital product.
type OwnershipChecker interface {
	HasAccessToProduct(ctx context.Context, userID, productID int64) (bool, error)
}

// DownloadHandler handles authorized digital file downloads.
type DownloadHandler struct {
	fileRepo         domain.ProductFileRepository
	ownershipChecker OwnershipChecker
	storage          *storage.Storage
}

// NewDownloadHandler constructs a new DownloadHandler.
func NewDownloadHandler(
	fileRepo domain.ProductFileRepository,
	ownershipChecker OwnershipChecker,
	storage *storage.Storage,
) *DownloadHandler {
	return &DownloadHandler{
		fileRepo:         fileRepo,
		ownershipChecker: ownershipChecker,
		storage:          storage,
	}
}

// DownloadFile securely streams a purchased digital product file.
// Enforces:
// 1. User must be authenticated.
// 2. User must have a paid order for the product (or be an admin).
// 3. File must exist in database and on disk.
func (h *DownloadHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
		return
	}

	fileIDStr := chi.URLParam(r, "fileID")
	fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 1. Retrieve file metadata
	file, err := h.fileRepo.FindByID(r.Context(), fileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 2. Verify authorization: admin or paid customer
	if !user.IsAdmin() {
		hasPurchased, err := h.ownershipChecker.HasAccessToProduct(r.Context(), user.ID, file.ProductID)
		if err != nil {
			http.Error(w, "Failed to verify purchase authorization", http.StatusInternalServerError)
			return
		}
		if !hasPurchased {
			http.Error(w, "Forbidden: You have not purchased this digital product", http.StatusForbidden)
			return
		}
	}

	// 3. Open file from secure storage
	diskFile, err := h.storage.Open(file.StoragePath)
	if err != nil {
		http.Error(w, "Product file could not be retrieved from storage", http.StatusInternalServerError)
		return
	}
	defer diskFile.Close()

	// 4. Set secure download headers
	mimeType := file.MIMEType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.OriginalName))
	if file.FileSize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.FileSize, 10))
	}

	// 5. Stream content to client
	_, _ = io.Copy(w, diskFile)
}

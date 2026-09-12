package storage_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/yusqohid/go-htmx-ecommerce/internal/storage"
)

func TestStorageSaveOpenDelete(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	content := []byte("Testing secure digital product file binary content")
	key, size, checksum, err := store.Save(bytes.NewReader(content), "document.pdf")
	if err != nil {
		t.Fatalf("failed to save file: %v", err)
	}

	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}

	if checksum == "" {
		t.Errorf("expected non-empty checksum")
	}

	// 1. Open and verify content
	file, err := store.Open(key)
	if err != nil {
		t.Fatalf("failed to open stored file: %v", err)
	}
	defer file.Close()

	readBytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("failed to read stored file: %v", err)
	}
	if !bytes.Equal(readBytes, content) {
		t.Errorf("content mismatch")
	}

	// 2. Path traversal attack attempt
	_, err = store.Open("../../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected path traversal security error, got %v", err)
	}

	// 3. Delete file
	if err := store.Delete(key); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	// Should no longer exist
	_, err = store.Open(key)
	if err == nil {
		t.Errorf("expected error opening deleted file, got nil")
	}
}

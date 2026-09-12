package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Storage manages secure filesystem storage for uploaded digital product files.
type Storage struct {
	basePath string
}

// New creates a new Storage manager and ensures the base directory exists.
func New(basePath string) (*Storage, error) {
	cleanPath := filepath.Clean(basePath)
	if err := os.MkdirAll(cleanPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create storage directory %s: %w", cleanPath, err)
	}
	return &Storage{basePath: cleanPath}, nil
}

// Save securely writes a digital product stream to disk.
// It generates a safe internal key, calculates the SHA-256 checksum, and counts the bytes.
func (s *Storage) Save(src io.Reader, originalFilename string) (storageKey string, size int64, checksum string, err error) {
	// Generate random 16-byte hex key to prevent path traversal or predictable URLs
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", 0, "", fmt.Errorf("failed to generate random storage key: %w", err)
	}
	randomKey := hex.EncodeToString(randomBytes)

	// Organize subdirectories by year/month to prevent too many files in one directory
	now := time.Now().UTC()
	subDir := filepath.Join(fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()))
	dirPath := filepath.Join(s.basePath, subDir)
	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return "", 0, "", fmt.Errorf("failed to create storage subfolder: %w", err)
	}

	storageKey = filepath.Join(subDir, randomKey+".bin")
	fullPath := filepath.Join(s.basePath, storageKey)

	dst, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to create storage file: %w", err)
	}
	defer func() {
		_ = dst.Close()
		if err != nil {
			_ = os.Remove(fullPath)
		}
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(dst, hasher)

	written, err := io.Copy(writer, src)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to write file content to storage: %w", err)
	}

	checksum = hex.EncodeToString(hasher.Sum(nil))
	return storageKey, written, checksum, nil
}

// Open opens a stored file safely, verifying that the target does not escape the basePath.
func (s *Storage) Open(storageKey string) (*os.File, error) {
	fullPath := filepath.Clean(filepath.Join(s.basePath, storageKey))
	if !strings.HasPrefix(fullPath, s.basePath) {
		return nil, fmt.Errorf("security violation: path traversal attempt detected")
	}

	return os.Open(fullPath)
}

// Delete removes a stored file from disk.
func (s *Storage) Delete(storageKey string) error {
	fullPath := filepath.Clean(filepath.Join(s.basePath, storageKey))
	if !strings.HasPrefix(fullPath, s.basePath) {
		return fmt.Errorf("security violation: path traversal attempt detected")
	}

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file from storage: %w", err)
	}
	return nil
}

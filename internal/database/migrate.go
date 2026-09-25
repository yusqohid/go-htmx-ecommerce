package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strings"
)

// MigrateUp executes all pending SQL migrations found in fsys in alphabetical order.
func MigrateUp(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	// Obtain a dedicated connection to hold session-scoped PostgreSQL advisory lock
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to obtain database connection for migration: %w", err)
	}
	defer conn.Close()

	// 724189312 is a unique 64-bit identifier for Sellora schema migrations lock
	const migrationLockID = 724189312
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("failed to acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)
	}()

	// Create schema_migrations tracking table if it doesn't exist
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	if _, err := conn.ExecContext(ctx, createTableQuery); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}
	// Read migration files from fsys
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var sqlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, file := range sqlFiles {
		version := filepath.Base(file)

		var exists bool
		query := `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`
		if err := conn.QueryRowContext(ctx, query, version).Scan(&exists); err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", version, err)
		}
		if exists {
			continue
		}

		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		log.Printf("Applying migration: %s ...", version)

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", version, err)
		}

		recordQuery := `INSERT INTO schema_migrations (version) VALUES ($1)`
		if _, err := tx.ExecContext(ctx, recordQuery, version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", version, err)
		}

		log.Printf("Successfully applied migration: %s", version)
	}

	return nil
}

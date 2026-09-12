package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// PostgresUserRepository implements domain.UserRepository using PostgreSQL.
type PostgresUserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new PostgresUserRepository.
func NewUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

// Create inserts a new user into the database.
func (r *PostgresUserRepository) Create(ctx context.Context, u *domain.User) error {
	query := `
		INSERT INTO users (email, name, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at;
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query,
		strings.ToLower(strings.TrimSpace(u.Email)),
		strings.TrimSpace(u.Name),
		u.PasswordHash,
		u.Role,
		now,
		now,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "idx_users_email") || strings.Contains(err.Error(), "users_email_key") {
			return fmt.Errorf("%w: user with email %s already exists", domain.ErrConflict, u.Email)
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// FindByID retrieves a user by their unique primary key ID.
func (r *PostgresUserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `
		SELECT id, email, name, password_hash, role, created_at, updated_at
		FROM users
		WHERE id = $1;
	`
	u := &domain.User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	return u, nil
}

// FindByEmail retrieves a user by their unique email address.
func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, name, password_hash, role, created_at, updated_at
		FROM users
		WHERE email = $1;
	`
	u := &domain.User{}
	err := r.db.QueryRowContext(ctx, query, strings.ToLower(strings.TrimSpace(email))).Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	return u, nil
}

// Update modifies user details.
func (r *PostgresUserRepository) Update(ctx context.Context, u *domain.User) error {
	query := `
		UPDATE users
		SET name = $1, email = $2, password_hash = $3, role = $4, updated_at = $5
		WHERE id = $6;
	`
	u.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query,
		strings.TrimSpace(u.Name),
		strings.ToLower(strings.TrimSpace(u.Email)),
		u.PasswordHash,
		u.Role,
		u.UpdatedAt,
		u.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// PostgresSessionRepository implements domain.SessionRepository using PostgreSQL.
type PostgresSessionRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a new PostgresSessionRepository.
func NewSessionRepository(db *sql.DB) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

// Create stores a new user session.
func (r *PostgresSessionRepository) Create(ctx context.Context, s *domain.Session) error {
	query := `
		INSERT INTO sessions (token, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4);
	`
	s.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, query, s.Token, s.UserID, s.ExpiresAt, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}
	return nil
}

// FindByToken retrieves an active session by token.
func (r *PostgresSessionRepository) FindByToken(ctx context.Context, token string) (*domain.Session, error) {
	query := `
		SELECT token, user_id, expires_at, created_at
		FROM sessions
		WHERE token = $1;
	`
	s := &domain.Session{}
	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&s.Token,
		&s.UserID,
		&s.ExpiresAt,
		&s.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find session by token: %w", err)
	}

	return s, nil
}

// DeleteByToken removes a session by its token.
func (r *PostgresSessionRepository) DeleteByToken(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE token = $1;`
	_, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// DeleteExpired removes all expired sessions from the database.
func (r *PostgresSessionRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < $1;`
	_, err := r.db.ExecContext(ctx, query, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	return nil
}

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionDuration = 7 * 24 * time.Hour
	BcryptCost      = 12
)

// Service defines auth operations and business logic.
type Service struct {
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
}

// NewService creates a new auth Service instance.
func NewService(userRepo domain.UserRepository, sessionRepo domain.SessionRepository) *Service {
	return &Service{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

// RegisterCustomer registers a new customer account and starts a session.
func (s *Service) RegisterCustomer(ctx context.Context, name, email, password string) (*domain.User, *domain.Session, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" {
		return nil, nil, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return nil, nil, fmt.Errorf("%w: invalid email address format", domain.ErrInvalidInput)
	}

	if len(password) < 8 {
		return nil, nil, fmt.Errorf("%w: password must be at least 8 characters long", domain.ErrInvalidInput)
	}

	// Check for existing user
	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		return nil, nil, fmt.Errorf("%w: user with email %s already exists", domain.ErrConflict, email)
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RoleCustomer,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, nil, err
	}

	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session after registration: %w", err)
	}

	return user, session, nil
}

// CreateAdmin creates an admin account (typically for initial seeding or CLI).
func (s *Service) CreateAdmin(ctx context.Context, name, email, password string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" || email == "" || len(password) < 8 {
		return nil, fmt.Errorf("%w: valid name, email, and minimum 8-char password required", domain.ErrInvalidInput)
	}

	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: admin user already exists", domain.ErrConflict)
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("failed checking existing user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	admin := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RoleAdmin,
	}

	if err := s.userRepo.Create(ctx, admin); err != nil {
		return nil, err
	}

	return admin, nil
}

// Login verifies credentials and returns the user with a newly generated session.
func (s *Service) Login(ctx context.Context, email, password string) (*domain.User, *domain.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// Use uniform error message to prevent user enumeration
		return nil, nil, domain.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, domain.ErrUnauthorized
	}

	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	return user, session, nil
}

// AuthenticateToken verifies the session token and returns the authenticated user.
func (s *Service) AuthenticateToken(ctx context.Context, token string) (*domain.User, error) {
	if token == "" {
		return nil, domain.ErrUnauthorized
	}

	session, err := s.sessionRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if session.IsExpired() {
		_ = s.sessionRepo.DeleteByToken(ctx, token)
		return nil, domain.ErrUnauthorized
	}

	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	return user, nil
}

// Logout deletes the session associated with the provided token.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessionRepo.DeleteByToken(ctx, token)
}

// ChangePassword verifies the user's current password and securely hashes and saves the new password.
func (s *Service) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("%w: new password must be at least 8 characters long", domain.ErrInvalidInput)
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return domain.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return domain.ErrUnauthorized
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), BcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = string(hash)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes all stale, expired sessions from the persistence store.
func (s *Service) CleanupExpiredSessions(ctx context.Context) error {
	return s.sessionRepo.DeleteExpired(ctx)
}
func (s *Service) createSession(ctx context.Context, userID int64) (*domain.Session, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	session := &domain.Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(SessionDuration),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

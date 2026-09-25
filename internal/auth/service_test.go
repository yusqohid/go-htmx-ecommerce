package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// MockUserRepository is an in-memory implementation of domain.UserRepository for unit testing.
type MockUserRepository struct {
	users  map[int64]*domain.User
	byMail map[string]*domain.User
	nextID int64
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:  make(map[int64]*domain.User),
		byMail: make(map[string]*domain.User),
		nextID: 1,
	}
}

func (m *MockUserRepository) Create(ctx context.Context, u *domain.User) error {
	if _, exists := m.byMail[u.Email]; exists {
		return domain.ErrConflict
	}
	u.ID = m.nextID
	m.nextID++
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	m.users[u.ID] = u
	m.byMail[u.Email] = u
	return nil
}

func (m *MockUserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, ok := m.byMail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (m *MockUserRepository) Update(ctx context.Context, u *domain.User) error {
	if _, ok := m.users[u.ID]; !ok {
		return domain.ErrNotFound
	}
	m.users[u.ID] = u
	m.byMail[u.Email] = u
	return nil
}

// MockSessionRepository is an in-memory implementation of domain.SessionRepository for unit testing.
type MockSessionRepository struct {
	sessions map[string]*domain.Session
}

func NewMockSessionRepository() *MockSessionRepository {
	return &MockSessionRepository{
		sessions: make(map[string]*domain.Session),
	}
}

func (m *MockSessionRepository) Create(ctx context.Context, s *domain.Session) error {
	m.sessions[s.Token] = s
	return nil
}

func (m *MockSessionRepository) FindByToken(ctx context.Context, token string) (*domain.Session, error) {
	s, ok := m.sessions[token]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (m *MockSessionRepository) DeleteByToken(ctx context.Context, token string) error {
	delete(m.sessions, token)
	return nil
}

func (m *MockSessionRepository) DeleteExpired(ctx context.Context) error {
	now := time.Now()
	for token, s := range m.sessions {
		if now.After(s.ExpiresAt) {
			delete(m.sessions, token)
		}
	}
	return nil
}

func TestRegisterCustomer(t *testing.T) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	ctx := context.Background()

	// 1. Success case
	user, session, err := service.RegisterCustomer(ctx, "Alice", "alice@example.com", "SecurePassword123")
	if err != nil {
		t.Fatalf("expected registration to succeed, got %v", err)
	}

	if user.ID == 0 || user.Email != "alice@example.com" || user.Role != domain.RoleCustomer {
		t.Errorf("unexpected user values: %+v", user)
	}

	if session.Token == "" || session.UserID != user.ID {
		t.Errorf("unexpected session: %+v", session)
	}

	// 2. Duplicate email should fail with ErrConflict
	_, _, err = service.RegisterCustomer(ctx, "Alice 2", "alice@example.com", "AnotherPassword123")
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict for duplicate email, got %v", err)
	}

	// 3. Short password should fail
	_, _, err = service.RegisterCustomer(ctx, "Bob", "bob@example.com", "short")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for short password, got %v", err)
	}

	// 4. Invalid email format should fail
	_, _, err = service.RegisterCustomer(ctx, "Bob", "invalid-email", "SecurePassword123")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for invalid email, got %v", err)
	}
}

func TestLoginAndAuthenticateToken(t *testing.T) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	ctx := context.Background()

	// Register user first
	rawPassword := "Password12345"
	_, _, err := service.RegisterCustomer(ctx, "Charlie", "charlie@example.com", rawPassword)
	if err != nil {
		t.Fatalf("failed to setup test user: %v", err)
	}

	// 1. Successful Login
	user, session, err := service.Login(ctx, "charlie@example.com", rawPassword)
	if err != nil {
		t.Fatalf("expected login to succeed, got %v", err)
	}
	if user.Email != "charlie@example.com" || session.Token == "" {
		t.Errorf("invalid login response: user=%+v, session=%+v", user, session)
	}

	// 2. Authenticate token
	authUser, err := service.AuthenticateToken(ctx, session.Token)
	if err != nil {
		t.Fatalf("expected token authentication to succeed, got %v", err)
	}
	if authUser.ID != user.ID {
		t.Errorf("expected authenticated user ID %d, got %d", user.ID, authUser.ID)
	}

	// 3. Login with wrong password
	_, _, err = service.Login(ctx, "charlie@example.com", "WrongPassword!")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized on wrong password, got %v", err)
	}

	// 4. Login with non-existent email
	_, _, err = service.Login(ctx, "nobody@example.com", rawPassword)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized on unknown email, got %v", err)
	}

	// 5. Logout
	err = service.Logout(ctx, session.Token)
	if err != nil {
		t.Fatalf("expected logout to succeed, got %v", err)
	}

	// Token should now be invalid
	_, err = service.AuthenticateToken(ctx, session.Token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized after logout, got %v", err)
	}
}

func TestCreateAdmin(t *testing.T) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	ctx := context.Background()

	admin, err := service.CreateAdmin(ctx, "Admin User", "admin@store.com", "AdminPassword123!")
	if err != nil {
		t.Fatalf("expected admin creation to succeed, got %v", err)
	}

	if !admin.IsAdmin() {
		t.Errorf("expected user to have admin role, got %s", admin.Role)
	}

	// Attempting duplicate should fail
	_, err = service.CreateAdmin(ctx, "Admin 2", "admin@store.com", "AdminPassword123!")
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict for duplicate admin, got %v", err)
	}
}
func TestChangePassword(t *testing.T) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	ctx := context.Background()

	user, _, err := service.RegisterCustomer(ctx, "Bob", "bob@example.com", "OldPassword123")
	if err != nil {
		t.Fatalf("setup registration failed: %v", err)
	}

	// 1. New password too short
	err = service.ChangePassword(ctx, user.ID, "OldPassword123", "short")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for short password, got %v", err)
	}

	// 2. Wrong current password
	err = service.ChangePassword(ctx, user.ID, "WrongPassword123", "BrandNewPassword123")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for wrong current password, got %v", err)
	}

	// 3. Success
	err = service.ChangePassword(ctx, user.ID, "OldPassword123", "BrandNewPassword123")
	if err != nil {
		t.Fatalf("expected password change to succeed, got %v", err)
	}

	// 4. Old password should fail login
	_, _, err = service.Login(ctx, "bob@example.com", "OldPassword123")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected login with old password to fail, got %v", err)
	}

	// 5. New password should succeed login
	loggedInUser, _, err := service.Login(ctx, "bob@example.com", "BrandNewPassword123")
	if err != nil {
		t.Fatalf("expected login with new password to succeed, got %v", err)
	}
	if loggedInUser.ID != user.ID {
		t.Errorf("expected user ID %d, got %d", user.ID, loggedInUser.ID)
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	service := auth.NewService(userRepo, sessionRepo)
	ctx := context.Background()

	// Add an active session
	activeSession := &domain.Session{
		Token:     "active-token",
		UserID:    1,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	_ = sessionRepo.Create(ctx, activeSession)

	// Add an expired session
	expiredSession := &domain.Session{
		Token:     "expired-token",
		UserID:    1,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	_ = sessionRepo.Create(ctx, expiredSession)

	if err := service.CleanupExpiredSessions(ctx); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if _, err := sessionRepo.FindByToken(ctx, "expired-token"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected expired-token to be cleaned up, got %v", err)
	}

	if _, err := sessionRepo.FindByToken(ctx, "active-token"); err != nil {
		t.Errorf("expected active-token to remain, got %v", err)
	}
}

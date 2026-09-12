package product_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yusqohid/go-htmx-ecommerce/internal/auth"
	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
	"github.com/yusqohid/go-htmx-ecommerce/internal/product"
	"github.com/yusqohid/go-htmx-ecommerce/internal/storage"
	"github.com/yusqohid/go-htmx-ecommerce/internal/view"
	"github.com/yusqohid/go-htmx-ecommerce/web/templates"
)

func TestProductHandlerAuthorization(t *testing.T) {
	productRepo := NewMockProductRepository()
	fileRepo := NewMockProductFileRepository()
	tempDir := t.TempDir()
	storageMgr, _ := storage.New(tempDir)
	service := product.NewService(productRepo, fileRepo, storageMgr)
	viewRenderer := view.New(templates.FS, true)
	handler := product.NewHandler(service, nil, viewRenderer, "mock")

	r := chi.NewRouter()
	r.Use(auth.RequireAuth("/login"))
	r.Use(auth.RequireRole(domain.RoleAdmin))
	r.Get("/admin/products", handler.ListProducts)
	r.Post("/admin/products/{id}/toggle-status", handler.ToggleStatus)

	// 1. Unauthenticated request -> Redirect to /login
	req := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for unauthenticated user, got %d", rr.Code)
	}

	// 2. Customer accessing -> 403 Forbidden
	reqCust := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	customer := &domain.User{ID: 2, Role: domain.RoleCustomer}
	ctxCust := context.WithValue(reqCust.Context(), auth.UserFromContext(reqCust.Context()), customer)
	// Inject user into context using the actual middleware
	rrCust := httptest.NewRecorder()
	// Create mock service with customer
	userRepo := NewMockUserRepository()
	sessionRepo := NewMockSessionRepository()
	authService := auth.NewService(userRepo, sessionRepo)
	_, custSession, _ := authService.RegisterCustomer(context.Background(), "Cust", "c@c.com", "Password123")

	authMiddleware := auth.Authenticate(authService)
	reqCustWithCookie := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	reqCustWithCookie.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: custSession.Token})
	authMiddleware(r).ServeHTTP(rrCust, reqCustWithCookie)

	if rrCust.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for customer accessing admin products, got %d", rrCust.Code)
	}

	// 3. Admin accessing -> 200 OK
	_, _ = authService.CreateAdmin(context.Background(), "Admin", "admin@store.com", "Password123")
	_, adminSession, _ := authService.Login(context.Background(), "admin@store.com", "Password123")

	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: adminSession.Token})
	rrAdmin := httptest.NewRecorder()
	authMiddleware(r).ServeHTTP(rrAdmin, reqAdmin)

	if rrAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for admin accessing products, got %d", rrAdmin.Code)
	}

	_ = ctxCust
}

func TestToggleStatusHTMX(t *testing.T) {
	productRepo := NewMockProductRepository()
	fileRepo := NewMockProductFileRepository()
	tempDir := t.TempDir()
	storageMgr, _ := storage.New(tempDir)
	service := product.NewService(productRepo, fileRepo, storageMgr)
	viewRenderer := view.New(templates.FS, true)
	handler := product.NewHandler(service, nil, viewRenderer, "mock")

	prod, _ := service.CreateProduct(context.Background(), product.CreateProductInput{
		Name:  "Toggle Product",
		Price: 100000,
	})

	r := chi.NewRouter()
	r.Post("/admin/products/{id}/toggle-status", handler.ToggleStatus)

	req := httptest.NewRequest(http.MethodPost, "/admin/products/1/toggle-status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}

	body := rr.Body.String()
	// After toggling from draft, status should be Published
	if !strings.Contains(body, "Published") {
		t.Errorf("expected HTMX response button to contain 'Published', got %s", body)
	}
	_ = prod
}

// Mock auth repositories for testing
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
	u.ID = m.nextID
	m.nextID++
	m.users[u.ID] = u
	m.byMail[u.Email] = u
	return nil
}

func (m *MockUserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	return m.users[id], nil
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, ok := m.byMail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (m *MockUserRepository) Update(ctx context.Context, u *domain.User) error {
	m.users[u.ID] = u
	return nil
}

type MockSessionRepository struct {
	sessions map[string]*domain.Session
}

func NewMockSessionRepository() *MockSessionRepository {
	return &MockSessionRepository{sessions: make(map[string]*domain.Session)}
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
	return nil
}

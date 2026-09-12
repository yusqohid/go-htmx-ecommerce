package domain_test

import (
	"testing"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

func TestUserDomain(t *testing.T) {
	admin := domain.User{Role: domain.RoleAdmin}
	customer := domain.User{Role: domain.RoleCustomer}

	if !admin.IsAdmin() {
		t.Errorf("expected admin.IsAdmin() to be true")
	}

	if customer.IsAdmin() {
		t.Errorf("expected customer.IsAdmin() to be false")
	}
}

func TestProductDomain(t *testing.T) {
	publishedProd := domain.Product{Status: domain.StatusPublished}
	draftProd := domain.Product{Status: domain.StatusDraft}

	if !publishedProd.IsPublished() {
		t.Errorf("expected publishedProd.IsPublished() to be true")
	}

	if draftProd.IsPublished() {
		t.Errorf("expected draftProd.IsPublished() to be false")
	}
}

func TestOrderDomain(t *testing.T) {
	paidOrder := domain.Order{Status: domain.StatusPaid}
	pendingOrder := domain.Order{Status: domain.StatusPending}

	if !paidOrder.IsPaid() {
		t.Errorf("expected paidOrder.IsPaid() to be true")
	}

	if pendingOrder.IsPaid() {
		t.Errorf("expected pendingOrder.IsPaid() to be false")
	}
}

func TestSessionExpiration(t *testing.T) {
	expiredSession := domain.Session{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	activeSession := domain.Session{
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if !expiredSession.IsExpired() {
		t.Errorf("expected session to be expired")
	}

	if activeSession.IsExpired() {
		t.Errorf("expected session to be active (not expired)")
	}
}

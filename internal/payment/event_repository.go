package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// PostgresPaymentEventRepository implements domain.PaymentEventRepository using PostgreSQL.
type PostgresPaymentEventRepository struct {
	db *sql.DB
}

// NewPaymentEventRepository constructs a new PostgresPaymentEventRepository.
func NewPaymentEventRepository(db *sql.DB) *PostgresPaymentEventRepository {
	return &PostgresPaymentEventRepository{db: db}
}

// Record inserts a processed webhook payment event into the database.
func (r *PostgresPaymentEventRepository) Record(ctx context.Context, event *domain.PaymentEvent) error {
	query := `
		INSERT INTO payment_events (provider, event_id, event_type, order_reference, payload, processed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider, event_id) DO NOTHING
		RETURNING id, created_at;
	`
	now := time.Now().UTC()
	if event.ProcessedAt.IsZero() {
		event.ProcessedAt = now
	}

	err := r.db.QueryRowContext(ctx, query,
		event.Provider,
		event.EventID,
		event.EventType,
		event.OrderReference,
		event.Payload,
		event.ProcessedAt,
		now,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Duplicate event already recorded, ignore
			return nil
		}
		return fmt.Errorf("failed to record payment event: %w", err)
	}

	return nil
}

// Exists checks if an event from a provider with a specific event ID has already been processed.
func (r *PostgresPaymentEventRepository) Exists(ctx context.Context, provider, eventID string) (bool, error) {
	query := `
		SELECT 1 FROM payment_events
		WHERE provider = $1 AND event_id = $2
		LIMIT 1;
	`
	var exists int
	err := r.db.QueryRowContext(ctx, query, provider, eventID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check payment event existence: %w", err)
	}

	return true, nil
}

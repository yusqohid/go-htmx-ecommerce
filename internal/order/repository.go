package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

// PostgresOrderRepository implements domain.OrderRepository with PostgreSQL.
type PostgresOrderRepository struct {
	db *sql.DB
}

// NewOrderRepository constructs a new PostgresOrderRepository.
func NewOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

// Create inserts an order and its line items inside an atomic database transaction.
func (r *PostgresOrderRepository) Create(ctx context.Context, o *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin order transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now().UTC()
	queryOrder := `
		INSERT INTO orders (reference, customer_id, status, total_amount, currency, payment_provider, payment_reference, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at;
	`
	err = tx.QueryRowContext(ctx, queryOrder,
		o.Reference,
		o.CustomerID,
		o.Status,
		o.TotalAmount,
		o.Currency,
		o.PaymentProvider,
		o.PaymentReference,
		now,
		now,
	).Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	queryItem := `
		INSERT INTO order_items (order_id, product_id, product_name, price, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at;
	`
	for i := range o.Items {
		item := &o.Items[i]
		item.OrderID = o.ID
		err = tx.QueryRowContext(ctx, queryItem,
			item.OrderID,
			item.ProductID,
			item.ProductName,
			item.Price,
			now,
		).Scan(&item.ID, &item.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit order transaction: %w", err)
	}

	return nil
}

// FindByID retrieves an order by its ID, joining its customer and loading its line items.
func (r *PostgresOrderRepository) FindByID(ctx context.Context, id int64) (*domain.Order, error) {
	query := `
		SELECT o.id, o.reference, o.customer_id, o.status, o.total_amount, o.currency,
		       o.payment_provider, o.payment_reference, o.created_at, o.updated_at,
		       u.name, u.email
		FROM orders o
		JOIN users u ON o.customer_id = u.id
		WHERE o.id = $1;
	`
	o := &domain.Order{Customer: &domain.User{}}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&o.ID,
		&o.Reference,
		&o.CustomerID,
		&o.Status,
		&o.TotalAmount,
		&o.Currency,
		&o.PaymentProvider,
		&o.PaymentReference,
		&o.CreatedAt,
		&o.UpdatedAt,
		&o.Customer.Name,
		&o.Customer.Email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query order by id: %w", err)
	}
	o.Customer.ID = o.CustomerID

	items, err := r.findItemsByOrderID(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items

	return o, nil
}

// FindByReference retrieves an order by its public reference code.
func (r *PostgresOrderRepository) FindByReference(ctx context.Context, reference string) (*domain.Order, error) {
	query := `
		SELECT o.id, o.reference, o.customer_id, o.status, o.total_amount, o.currency,
		       o.payment_provider, o.payment_reference, o.created_at, o.updated_at,
		       u.name, u.email
		FROM orders o
		JOIN users u ON o.customer_id = u.id
		WHERE o.reference = $1;
	`
	o := &domain.Order{Customer: &domain.User{}}
	err := r.db.QueryRowContext(ctx, query, reference).Scan(
		&o.ID,
		&o.Reference,
		&o.CustomerID,
		&o.Status,
		&o.TotalAmount,
		&o.Currency,
		&o.PaymentProvider,
		&o.PaymentReference,
		&o.CreatedAt,
		&o.UpdatedAt,
		&o.Customer.Name,
		&o.Customer.Email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query order by reference: %w", err)
	}
	o.Customer.ID = o.CustomerID

	items, err := r.findItemsByOrderID(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items

	return o, nil
}

// ListByCustomerID retrieves all orders made by a specific customer.
func (r *PostgresOrderRepository) ListByCustomerID(ctx context.Context, customerID int64) ([]domain.Order, error) {
	query := `
		SELECT id, reference, customer_id, status, total_amount, currency,
		       payment_provider, payment_reference, created_at, updated_at
		FROM orders
		WHERE customer_id = $1
		ORDER BY created_at DESC;
	`
	rows, err := r.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query customer orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(
			&o.ID,
			&o.Reference,
			&o.CustomerID,
			&o.Status,
			&o.TotalAmount,
			&o.Currency,
			&o.PaymentProvider,
			&o.PaymentReference,
			&o.CreatedAt,
			&o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan customer order: %w", err)
		}
		orders = append(orders, o)
	}

	for i := range orders {
		items, err := r.findItemsByOrderID(ctx, orders[i].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load items for order %d: %w", orders[i].ID, err)
		}
		orders[i].Items = items
	}

	return orders, nil
}

// ListAll retrieves paginated orders for the admin order management dashboard.
func (r *PostgresOrderRepository) ListAll(ctx context.Context, limit, offset int) ([]domain.Order, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM orders;`
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	query := `
		SELECT o.id, o.reference, o.customer_id, o.status, o.total_amount, o.currency,
		       o.payment_provider, o.payment_reference, o.created_at, o.updated_at,
		       u.name, u.email
		FROM orders o
		JOIN users u ON o.customer_id = u.id
		ORDER BY o.created_at DESC
		LIMIT $1 OFFSET $2;
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query all orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		o.Customer = &domain.User{}
		if err := rows.Scan(
			&o.ID,
			&o.Reference,
			&o.CustomerID,
			&o.Status,
			&o.TotalAmount,
			&o.Currency,
			&o.PaymentProvider,
			&o.PaymentReference,
			&o.CreatedAt,
			&o.UpdatedAt,
			&o.Customer.Name,
			&o.Customer.Email,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan order: %w", err)
		}
		o.Customer.ID = o.CustomerID
		orders = append(orders, o)
	}

	for i := range orders {
		items, err := r.findItemsByOrderID(ctx, orders[i].ID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to load items for order %d: %w", orders[i].ID, err)
		}
		orders[i].Items = items
	}

	return orders, total, nil
}

// UpdateStatus updates the lifecycle state and payment reference of an order.
func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, id int64, status domain.OrderStatus, paymentReference string) error {
	query := `
		UPDATE orders
		SET status = $1, payment_reference = $2, updated_at = $3
		WHERE id = $4;
	`
	res, err := r.db.ExecContext(ctx, query, status, paymentReference, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// HasUserPurchasedProduct checks if a user has a completed 'paid' order containing the specified product.
func (r *PostgresOrderRepository) HasUserPurchasedProduct(ctx context.Context, userID, productID int64) (bool, error) {
	query := `
		SELECT 1
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		WHERE o.customer_id = $1 AND oi.product_id = $2 AND o.status = 'paid'
		LIMIT 1;
	`
	var exists int
	err := r.db.QueryRowContext(ctx, query, userID, productID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check user product purchase: %w", err)
	}

	return true, nil
}

func (r *PostgresOrderRepository) findItemsByOrderID(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	query := `
		SELECT id, order_id, product_id, product_name, price, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY id ASC;
	`
	rows, err := r.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.ProductID,
			&item.ProductName,
			&item.Price,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

package domain

import (
	"context"
	"time"
)

// OrderStatus represents the current state of an order.
type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusPaid      OrderStatus = "paid"
	StatusFailed    OrderStatus = "failed"
	StatusCancelled OrderStatus = "cancelled"
	StatusRefunded  OrderStatus = "refunded"
)

// Order represents a customer purchase transaction.
type Order struct {
	ID               int64       `json:"id"`
	Reference        string      `json:"reference"`
	CustomerID       int64       `json:"customer_id"`
	Customer         *User       `json:"customer,omitempty"`
	Status           OrderStatus `json:"status"`
	TotalAmount      int64       `json:"total_amount"` // final payable amount after discount
	DiscountAmount   int64       `json:"discount_amount"` // discount applied
	CouponID         *int64      `json:"coupon_id,omitempty"`
	Coupon           *Coupon     `json:"coupon,omitempty"`
	Currency         string      `json:"currency"`
	PaymentProvider  string      `json:"payment_provider"`
	PaymentReference string      `json:"payment_reference"`
	Items            []OrderItem `json:"items,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// IsPaid checks if the order has been successfully paid for.
func (o *Order) IsPaid() bool {
	return o.Status == StatusPaid
}

// Subtotal calculates the sum of all item prices before discounts.
func (o *Order) Subtotal() int64 {
	var sum int64
	for _, it := range o.Items {
		sum += it.Price
	}
	return sum
}

// OrderItem represents a line item within an order, capturing snapshot prices.
type OrderItem struct {
	ID          int64     `json:"id"`
	OrderID     int64     `json:"order_id"`
	ProductID   int64     `json:"product_id"`
	ProductName string    `json:"product_name"`
	Price       int64     `json:"price"` // price at the moment of order creation
	CreatedAt   time.Time `json:"created_at"`
}

// OrderRepository defines persistence operations for orders and their items.
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, id int64) (*Order, error)
	FindByReference(ctx context.Context, reference string) (*Order, error)
	ListByCustomerID(ctx context.Context, customerID int64) ([]Order, error)
	ListAll(ctx context.Context, limit, offset int) ([]Order, int, error)
	UpdateStatus(ctx context.Context, id int64, status OrderStatus, paymentReference string) error
	ProcessPaymentResult(ctx context.Context, orderID int64, status OrderStatus, paymentReference string, event *PaymentEvent) error
	HasUserPurchasedProduct(ctx context.Context, userID, productID int64) (bool, error)
}

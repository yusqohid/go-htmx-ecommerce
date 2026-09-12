package domain

import "errors"

var (
	// ErrNotFound is returned when an entity does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrUnauthorized is returned when authentication is missing or invalid.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when the user lacks required permissions.
	ErrForbidden = errors.New("forbidden")

	// ErrConflict is returned when an entity violates uniqueness (e.g. duplicate email/slug).
	ErrConflict = errors.New("resource conflict or already exists")

	// ErrInvalidInput is returned when validation fails.
	ErrInvalidInput = errors.New("invalid input data")

	// ErrPaymentFailed is returned when an external payment action fails.
	ErrPaymentFailed = errors.New("payment processing failed")

	// ErrOrderAlreadyPaid is returned when an operation expects an unpaid order.
	ErrOrderAlreadyPaid = errors.New("order is already paid")
)

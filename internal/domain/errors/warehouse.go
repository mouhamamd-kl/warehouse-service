package errors

import "errors"

var (
	ErrInventoryAlreadyExists = errors.New("inventory already exists")
	ErrInsufficientStock      = errors.New("insufficient available stock")

	ErrReservationNotFound            = errors.New("reservation not found")
	ErrReservationConflict            = errors.New("reservation is already in a different state")
	ErrReservationExpired             = errors.New("reservation has expired")
	ErrIdempotencyConflict            = errors.New("idempotency key was used with different data")
	ErrReservationBalanceInconsistent = errors.New("reservation balance inconsistency")

	ErrAdjustmentAlreadyExists = errors.New("stock adjustment already processed")
)

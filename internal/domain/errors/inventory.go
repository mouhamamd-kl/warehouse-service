package errors

import "errors"

var (
	ErrInventoryNotFound = errors.New("inventory not found")
	ErrInvalidProductID  = errors.New("product ID must be greater than zero")
)

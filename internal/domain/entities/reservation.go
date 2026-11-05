package entities

import "time"

type ReservationStatus string

const (
	ReservationActive    ReservationStatus = "active"
	ReservationReleased  ReservationStatus = "released"
	ReservationConfirmed ReservationStatus = "confirmed"
	ReservationExpired   ReservationStatus = "expired"
)

type ReservationItem struct {
	ProductID int64
	Quantity  int64
}

type Reservation struct {
	ID             string
	OrderID        string
	IdempotencyKey string
	Status         ReservationStatus
	Items          []ReservationItem
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

package inputs

type ReservationItemInput struct {
	ProductID int64
	Quantity  int64
}

type ReserveStockInput struct {
	OrderID          string
	IdempotencyKey   string
	Items            []ReservationItemInput
	ExpiresInSeconds int64
}

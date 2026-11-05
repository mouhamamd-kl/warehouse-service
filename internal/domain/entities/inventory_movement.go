package entities

import "time"

type MovementType string

const (
	MovementStockAdded   MovementType = "stock_added"
	MovementStockRemoved MovementType = "stock_removed"
	MovementReserved     MovementType = "reserved"
	MovementReleased     MovementType = "released"
	MovementSold         MovementType = "sold"
)

type InventoryMovement struct {
	ID             int64
	InventoryID    int64
	ProductID      int64
	Type           MovementType
	Quantity       int64
	QuantityBefore int64
	QuantityAfter  int64
	Reason         string
	ReferenceID    string
	CreatedAt      time.Time
}

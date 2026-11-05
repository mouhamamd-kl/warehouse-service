package outputs

import "time"

type InventoryOutput struct {
	ID                int64
	ProductID         int64
	QuantityOnHand    int64
	QuantityReserved  int64
	AvailableQuantity int64
	LowStockThreshold int64
	IsLowStock        bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

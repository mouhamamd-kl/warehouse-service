package entities

import "time"

type Inventory struct {
	ID                int64
	ProductID         int64
	QuantityOnHand    int64
	QuantityReserved  int64
	LowStockThreshold int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (i *Inventory) AvailableQuantity() int64 {
	return i.QuantityOnHand - i.QuantityReserved
}

func (i *Inventory) IsLowStock() bool {
	return i.AvailableQuantity() <= i.LowStockThreshold
}

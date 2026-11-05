package inputs

type CreateInventoryInput struct {
	ProductID         int64
	QuantityOnHand    int64
	LowStockThreshold int64
}

type AdjustStockInput struct {
	ProductID      int64
	Quantity       int64
	Operation      string
	Reason         string
	ReferenceID    string
	IdempotencyKey string
}

type ListInventoryInput struct {
	Page  int
	Limit int
}

type GetMovementsInput struct {
	ProductID int64
	Page      int
	Limit     int
}

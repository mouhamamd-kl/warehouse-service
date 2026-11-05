package repositories

import (
	"context"

	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
)

type InventoryRepository interface {
	FindByProductID(
		ctx context.Context,
		productID int64,
	) (*entities.Inventory, error)
}

package usecases

import (
	"context"

	"github.com/mouhamamd-kl/warehouse-service/internal/application/contracts/outputs"
	"github.com/mouhamamd-kl/warehouse-service/internal/application/ports/repositories"
	domainerrors "github.com/mouhamamd-kl/warehouse-service/internal/domain/errors"
)

type GetInventoryUseCase struct {
	inventoryRepository repositories.InventoryRepository
}

func NewGetInventoryUseCase(
	inventoryRepository repositories.InventoryRepository,
) *GetInventoryUseCase {
	return &GetInventoryUseCase{
		inventoryRepository: inventoryRepository,
	}
}

func (uc *GetInventoryUseCase) Execute(
	ctx context.Context,
	productID int64,
) (*outputs.InventoryOutput, error) {
	if productID <= 0 {
		return nil, domainerrors.ErrInvalidProductID
	}

	inventory, err := uc.inventoryRepository.FindByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	return &outputs.InventoryOutput{
		ID:                inventory.ID,
		ProductID:         inventory.ProductID,
		QuantityOnHand:    inventory.QuantityOnHand,
		QuantityReserved:  inventory.QuantityReserved,
		AvailableQuantity: inventory.AvailableQuantity(),
		LowStockThreshold: inventory.LowStockThreshold,
		IsLowStock:        inventory.IsLowStock(),
		CreatedAt:         inventory.CreatedAt,
		UpdatedAt:         inventory.UpdatedAt,
	}, nil
}

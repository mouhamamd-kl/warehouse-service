package usecases

import (
	"context"
	"errors"
	"strings"

	"github.com/mouhamamd-kl/warehouse-service/internal/application/contracts/inputs"
	"github.com/mouhamamd-kl/warehouse-service/internal/application/ports/repositories"
	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
)

var (
	ErrInvalidInput     = errors.New("invalid input")
	ErrInvalidOperation = errors.New("operation must be add or remove")
)

type WarehouseUseCase struct {
	repository repositories.WarehouseRepository
}

func NewWarehouseUseCase(
	repository repositories.WarehouseRepository,
) *WarehouseUseCase {
	return &WarehouseUseCase{repository: repository}
}

func (uc *WarehouseUseCase) CreateInventory(
	ctx context.Context,
	input inputs.CreateInventoryInput,
) (*entities.Inventory, error) {
	if input.ProductID <= 0 ||
		input.QuantityOnHand < 0 ||
		input.LowStockThreshold < 0 {
		return nil, ErrInvalidInput
	}

	inventory := &entities.Inventory{
		ProductID:         input.ProductID,
		QuantityOnHand:    input.QuantityOnHand,
		LowStockThreshold: input.LowStockThreshold,
	}

	return uc.repository.CreateInventory(ctx, inventory)
}

func (uc *WarehouseUseCase) GetInventory(
	ctx context.Context,
	productID int64,
) (*entities.Inventory, error) {
	if productID <= 0 {
		return nil, ErrInvalidInput
	}

	return uc.repository.FindByProductID(ctx, productID)
}

func (uc *WarehouseUseCase) GetAllInventory(
	ctx context.Context,
	page, limit int,
) ([]entities.Inventory, int64, error) {
	page, limit = normalizePagination(page, limit)

	return uc.repository.FindAll(ctx, page, limit)
}

func (uc *WarehouseUseCase) GetLowStock(
	ctx context.Context,
	page, limit int,
) ([]entities.Inventory, int64, error) {
	page, limit = normalizePagination(page, limit)

	return uc.repository.FindLowStock(ctx, page, limit)
}

func (uc *WarehouseUseCase) AdjustStock(
	ctx context.Context,
	input inputs.AdjustStockInput,
) (*entities.Inventory, error) {
	if input.ProductID <= 0 ||
		input.Quantity <= 0 ||
		strings.TrimSpace(input.Reason) == "" ||
		strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, ErrInvalidInput
	}

	if input.Operation != "add" && input.Operation != "remove" {
		return nil, ErrInvalidOperation
	}

	return uc.repository.AdjustStock(
		ctx,
		input.ProductID,
		input.Quantity,
		input.Operation,
		input.Reason,
		input.ReferenceID,
		input.IdempotencyKey,
	)
}

func (uc *WarehouseUseCase) GetMovements(
	ctx context.Context,
	input inputs.GetMovementsInput,
) ([]entities.InventoryMovement, int64, error) {
	if input.ProductID <= 0 {
		return nil, 0, ErrInvalidInput
	}

	page, limit := normalizePagination(input.Page, input.Limit)

	return uc.repository.GetMovements(
		ctx,
		input.ProductID,
		page,
		limit,
	)
}

func normalizePagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	return page, limit
}

package repositories

import (
	"context"

	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
)

type WarehouseRepository interface {
	CreateInventory(
		ctx context.Context,
		inventory *entities.Inventory,
	) (*entities.Inventory, error)

	FindByProductID(
		ctx context.Context,
		productID int64,
	) (*entities.Inventory, error)

	FindAll(
		ctx context.Context,
		page, limit int,
	) ([]entities.Inventory, int64, error)

	FindLowStock(
		ctx context.Context,
		page, limit int,
	) ([]entities.Inventory, int64, error)

	AdjustStock(
		ctx context.Context,
		productID int64,
		quantity int64,
		operation string,
		reason string,
		referenceID string,
		idempotencyKey string,
	) (*entities.Inventory, error)

	GetMovements(
		ctx context.Context,
		productID int64,
		page, limit int,
	) ([]entities.InventoryMovement, int64, error)

	ReserveStock(
		ctx context.Context,
		reservation *entities.Reservation,
	) (*entities.Reservation, error)

	FindReservation(
		ctx context.Context,
		reservationID string,
	) (*entities.Reservation, error)

	ReleaseReservation(
		ctx context.Context,
		reservationID string,
		reason string,
	) (*entities.Reservation, error)

	ConfirmReservation(
		ctx context.Context,
		reservationID string,
	) (*entities.Reservation, error)
}

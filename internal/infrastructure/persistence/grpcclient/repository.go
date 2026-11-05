// Package grpcclient implements the application repository ports on top of
// the generated warehouse.v1 gRPC client, so this service can be run with
// its data coming from a remote warehouse service instead of its own
// PostgreSQL database.
package grpcclient

import (
	"context"
	"time"

	warehousev1 "github.com/mouhamamd-kl/warehouse-service/gen/warehouse/v1"

	"github.com/mouhamamd-kl/warehouse-service/internal/application/ports/repositories"
	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
)

// Compile-time proof that the adapter satisfies the repository port.
var _ repositories.WarehouseRepository = (*Repository)(nil)

type Repository struct {
	client  warehousev1.WarehouseServiceClient
	timeout time.Duration
}

// NewRepository builds a repository backed by a warehouse.v1 gRPC client.
// Each call gets the given timeout, unless the caller context already
// carries a shorter deadline.
func NewRepository(
	client warehousev1.WarehouseServiceClient,
	timeout time.Duration,
) *Repository {
	return &Repository{
		client:  client,
		timeout: timeout,
	}
}

func (r *Repository) call(
	ctx context.Context,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.timeout)
}

func (r *Repository) CreateInventory(
	ctx context.Context,
	inventory *entities.Inventory,
) (*entities.Inventory, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.CreateInventory(ctx, &warehousev1.CreateInventoryRequest{
		ProductId:         inventory.ProductID,
		QuantityOnHand:    inventory.QuantityOnHand,
		LowStockThreshold: inventory.LowStockThreshold,
	})
	if err != nil {
		return nil, decodeCreateInventoryError(err)
	}

	return inventoryFromProto(response)
}

func (r *Repository) FindByProductID(
	ctx context.Context,
	productID int64,
) (*entities.Inventory, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.GetInventory(ctx, &warehousev1.GetInventoryRequest{
		ProductId: productID,
	})
	if err != nil {
		return nil, decodeGetInventoryError(err)
	}

	return inventoryFromProto(response)
}

func (r *Repository) FindAll(
	ctx context.Context,
	page, limit int,
) ([]entities.Inventory, int64, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.GetAllInventory(ctx, &warehousev1.ListInventoryRequest{
		Page:  int32(page),
		Limit: int32(limit),
	})
	if err != nil {
		return nil, 0, decodeGetInventoryError(err)
	}

	items, err := inventoryListFromProto(response.GetItems())
	if err != nil {
		return nil, 0, err
	}

	return items, response.GetTotal(), nil
}

func (r *Repository) FindLowStock(
	ctx context.Context,
	page, limit int,
) ([]entities.Inventory, int64, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.GetLowStock(ctx, &warehousev1.ListInventoryRequest{
		Page:  int32(page),
		Limit: int32(limit),
	})
	if err != nil {
		return nil, 0, decodeGetInventoryError(err)
	}

	items, err := inventoryListFromProto(response.GetItems())
	if err != nil {
		return nil, 0, err
	}

	return items, response.GetTotal(), nil
}

func (r *Repository) AdjustStock(
	ctx context.Context,
	productID int64,
	quantity int64,
	operation string,
	reason string,
	referenceID string,
	idempotencyKey string,
) (*entities.Inventory, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.AdjustStock(ctx, &warehousev1.AdjustStockRequest{
		ProductId:      productID,
		Quantity:       quantity,
		Operation:      operation,
		Reason:         reason,
		ReferenceId:    referenceID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return nil, decodeAdjustStockError(err)
	}

	return inventoryFromProto(response)
}

func (r *Repository) GetMovements(
	ctx context.Context,
	productID int64,
	page, limit int,
) ([]entities.InventoryMovement, int64, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.GetMovements(ctx, &warehousev1.GetMovementsRequest{
		ProductId: productID,
		Page:      int32(page),
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, 0, decodeGetInventoryError(err)
	}

	items, err := movementListFromProto(response.GetItems())
	if err != nil {
		return nil, 0, err
	}

	return items, response.GetTotal(), nil
}

func (r *Repository) ReserveStock(
	ctx context.Context,
	reservation *entities.Reservation,
) (*entities.Reservation, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	items := make([]*warehousev1.ReservationItem, 0, len(reservation.Items))

	for _, item := range reservation.Items {
		items = append(items, &warehousev1.ReservationItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	response, err := r.client.ReserveStock(ctx, &warehousev1.ReserveStockRequest{
		OrderId:          reservation.OrderID,
		IdempotencyKey:   reservation.IdempotencyKey,
		Items:            items,
		ExpiresInSeconds: expiresInSeconds(reservation.ExpiresAt),
	})
	if err != nil {
		return nil, decodeReserveStockError(err)
	}

	return reservationFromProto(response)
}

func (r *Repository) FindReservation(
	ctx context.Context,
	reservationID string,
) (*entities.Reservation, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.GetReservation(ctx, &warehousev1.GetReservationRequest{
		ReservationId: reservationID,
	})
	if err != nil {
		return nil, decodeFindReservationError(err)
	}

	return reservationFromProto(response)
}

func (r *Repository) ReleaseReservation(
	ctx context.Context,
	reservationID string,
	reason string,
) (*entities.Reservation, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.ReleaseReservation(ctx, &warehousev1.ReleaseReservationRequest{
		ReservationId: reservationID,
		Reason:        reason,
	})
	if err != nil {
		return nil, decodeReleaseConfirmError(err)
	}

	return reservationFromProto(response)
}

func (r *Repository) ConfirmReservation(
	ctx context.Context,
	reservationID string,
) (*entities.Reservation, error) {
	ctx, cancel := r.call(ctx)
	defer cancel()

	response, err := r.client.ConfirmReservation(ctx, &warehousev1.ConfirmReservationRequest{
		ReservationId: reservationID,
	})
	if err != nil {
		return nil, decodeReleaseConfirmError(err)
	}

	return reservationFromProto(response)
}

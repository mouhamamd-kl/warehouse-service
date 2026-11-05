package grpc

import (
	"context"
	"errors"
	"time"

	warehousev1 "github.com/mouhamamd-kl/warehouse-service/gen/warehouse/v1"
	"github.com/mouhamamd-kl/warehouse-service/internal/application/contracts/inputs"
	"github.com/mouhamamd-kl/warehouse-service/internal/application/usecases"
	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
	domainerrors "github.com/mouhamamd-kl/warehouse-service/internal/domain/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	grpcstatus "google.golang.org/grpc/status"
)

type WarehouseHandler struct {
	warehousev1.UnimplementedWarehouseServiceServer

	warehouse    *usecases.WarehouseUseCase
	reservations *ReservationHandler
}

func NewWarehouseHandler(
	warehouse *usecases.WarehouseUseCase,
	reservations *ReservationHandler,
) *WarehouseHandler {
	return &WarehouseHandler{
		warehouse:    warehouse,
		reservations: reservations,
	}
}

func inventoryResponse(
	item *entities.Inventory,
) *warehousev1.InventoryResponse {
	return &warehousev1.InventoryResponse{
		Id:                item.ID,
		ProductId:         item.ProductID,
		QuantityOnHand:    item.QuantityOnHand,
		QuantityReserved:  item.QuantityReserved,
		AvailableQuantity: item.AvailableQuantity(),
		LowStockThreshold: item.LowStockThreshold,
		IsLowStock:        item.IsLowStock(),
		CreatedAt:         item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.Format(time.RFC3339),
	}
}

func movementResponse(
	item entities.InventoryMovement,
) *warehousev1.MovementResponse {
	return &warehousev1.MovementResponse{
		Id:             item.ID,
		InventoryId:    item.InventoryID,
		ProductId:      item.ProductID,
		Type:           string(item.Type),
		Quantity:       item.Quantity,
		QuantityBefore: item.QuantityBefore,
		QuantityAfter:  item.QuantityAfter,
		Reason:         item.Reason,
		ReferenceId:    item.ReferenceID,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
	}
}

func (h *WarehouseHandler) CreateInventory(
	ctx context.Context,
	req *warehousev1.CreateInventoryRequest,
) (*warehousev1.InventoryResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	item, err := h.warehouse.CreateInventory(ctx, inputs.CreateInventoryInput{
		ProductID:         req.GetProductId(),
		QuantityOnHand:    req.GetQuantityOnHand(),
		LowStockThreshold: req.GetLowStockThreshold(),
	})
	if err != nil {
		return nil, mapWarehouseError(err)
	}

	return inventoryResponse(item), nil
}

func (h *WarehouseHandler) GetInventory(
	ctx context.Context,
	req *warehousev1.GetInventoryRequest,
) (*warehousev1.InventoryResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	item, err := h.warehouse.GetInventory(ctx, req.GetProductId())
	if err != nil {
		return nil, mapWarehouseError(err)
	}

	return inventoryResponse(item), nil
}

func (h *WarehouseHandler) GetAllInventory(
	ctx context.Context,
	req *warehousev1.ListInventoryRequest,
) (*warehousev1.ListInventoryResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	items, total, err := h.warehouse.GetAllInventory(
		ctx,
		int(req.GetPage()),
		int(req.GetLimit()),
	)
	if err != nil {
		return nil, mapWarehouseError(err)
	}

	return &warehousev1.ListInventoryResponse{
		Items: mapInventoryItems(items),
		Total: total,
		Page:  req.GetPage(),
		Limit: req.GetLimit(),
	}, nil
}

func (h *WarehouseHandler) GetLowStock(
	ctx context.Context,
	req *warehousev1.ListInventoryRequest,
) (*warehousev1.ListInventoryResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	items, total, err := h.warehouse.GetLowStock(
		ctx,
		int(req.GetPage()),
		int(req.GetLimit()),
	)
	if err != nil {
		return nil, mapWarehouseError(err)
	}

	return &warehousev1.ListInventoryResponse{
		Items: mapInventoryItems(items),
		Total: total,
		Page:  req.GetPage(),
		Limit: req.GetLimit(),
	}, nil
}

func mapInventoryItems(
	items []entities.Inventory,
) []*warehousev1.InventoryResponse {
	result := make([]*warehousev1.InventoryResponse, 0, len(items))

	for i := range items {
		result = append(result, inventoryResponse(&items[i]))
	}

	return result
}

func (h *WarehouseHandler) AdjustStock(
	ctx context.Context,
	req *warehousev1.AdjustStockRequest,
) (*warehousev1.InventoryResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	item, err := h.warehouse.AdjustStock(ctx, inputs.AdjustStockInput{
		ProductID:      req.GetProductId(),
		Quantity:       req.GetQuantity(),
		Operation:      req.GetOperation(),
		Reason:         req.GetReason(),
		ReferenceID:    req.GetReferenceId(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, mapWarehouseError(err)
	}

	return inventoryResponse(item), nil
}

func (h *WarehouseHandler) GetMovements(
	ctx context.Context,
	req *warehousev1.GetMovementsRequest,
) (*warehousev1.GetMovementsResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	items, total, err := h.warehouse.GetMovements(ctx, inputs.GetMovementsInput{
		ProductID: req.GetProductId(),
		Page:      int(req.GetPage()),
		Limit:     int(req.GetLimit()),
	})
	if err != nil {
		return nil, mapWarehouseError(err)
	}

	result := make([]*warehousev1.MovementResponse, 0, len(items))

	for _, item := range items {
		result = append(result, movementResponse(item))
	}

	return &warehousev1.GetMovementsResponse{
		Items: result,
		Total: total,
	}, nil
}

// Forward reservation RPCs to the reservation handler.

func (h *WarehouseHandler) ReserveStock(
	ctx context.Context,
	req *warehousev1.ReserveStockRequest,
) (*warehousev1.ReservationResponse, error) {
	return h.reservations.ReserveStock(ctx, req)
}

func (h *WarehouseHandler) GetReservation(
	ctx context.Context,
	req *warehousev1.GetReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	return h.reservations.GetReservation(ctx, req)
}

func (h *WarehouseHandler) ReleaseReservation(
	ctx context.Context,
	req *warehousev1.ReleaseReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	return h.reservations.ReleaseReservation(ctx, req)
}

func (h *WarehouseHandler) ConfirmReservation(
	ctx context.Context,
	req *warehousev1.ConfirmReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	return h.reservations.ConfirmReservation(ctx, req)
}

func mapWarehouseError(err error) error {
	switch {
	case errors.Is(err, domainerrors.ErrInventoryNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domainerrors.ErrInventoryAlreadyExists),
		errors.Is(err, domainerrors.ErrAdjustmentAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, domainerrors.ErrInsufficientStock):
		return status.Error(codes.FailedPrecondition, err.Error())

	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

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

type ReservationHandler struct {
	reservations *usecases.ReservationUseCase
}

func NewReservationHandler(
	reservations *usecases.ReservationUseCase,
) *ReservationHandler {
	return &ReservationHandler{
		reservations: reservations,
	}
}

func reservationResponse(
	reservation *entities.Reservation,
) *warehousev1.ReservationResponse {
	items := make([]*warehousev1.ReservationItem, 0, len(reservation.Items))

	for _, item := range reservation.Items {
		items = append(items, &warehousev1.ReservationItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	response := &warehousev1.ReservationResponse{
		Id:        reservation.ID,
		OrderId:   reservation.OrderID,
		Status:    string(reservation.Status),
		Items:     items,
		CreatedAt: reservation.CreatedAt.Format(time.RFC3339),
	}

	if reservation.ExpiresAt != nil {
		response.ExpiresAt = reservation.ExpiresAt.Format(time.RFC3339)
	}

	return response
}

func (h *ReservationHandler) ReserveStock(
	ctx context.Context,
	req *warehousev1.ReserveStockRequest,
) (*warehousev1.ReservationResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	items := make([]inputs.ReservationItemInput, 0, len(req.GetItems()))

	for _, item := range req.GetItems() {
		if item == nil {
			return nil, grpcstatus.Error(codes.InvalidArgument, "reservation item is required")
		}

		items = append(items, inputs.ReservationItemInput{
			ProductID: item.GetProductId(),
			Quantity:  item.GetQuantity(),
		})
	}

	reservation, err := h.reservations.ReserveStock(ctx, inputs.ReserveStockInput{
		OrderID:          req.GetOrderId(),
		IdempotencyKey:   req.GetIdempotencyKey(),
		Items:            items,
		ExpiresInSeconds: req.GetExpiresInSeconds(),
	})

	if err != nil {
		return nil, mapReservationError(err)
	}

	return reservationResponse(reservation), nil
}

func (h *ReservationHandler) GetReservation(
	ctx context.Context,
	req *warehousev1.GetReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	reservation, err := h.reservations.GetReservation(ctx, req.GetReservationId())
	if err != nil {
		return nil, mapReservationError(err)
	}

	return reservationResponse(reservation), nil
}

func (h *ReservationHandler) ReleaseReservation(
	ctx context.Context,
	req *warehousev1.ReleaseReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	reservation, err := h.reservations.ReleaseReservation(
		ctx,
		req.GetReservationId(),
		req.GetReason(),
	)
	if err != nil {
		return nil, mapReservationError(err)
	}

	return reservationResponse(reservation), nil
}

func (h *ReservationHandler) ConfirmReservation(
	ctx context.Context,
	req *warehousev1.ConfirmReservationRequest,
) (*warehousev1.ReservationResponse, error) {
	if req == nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, "request is required")
	}

	reservation, err := h.reservations.ConfirmReservation(
		ctx,
		req.GetReservationId(),
	)
	if err != nil {
		return nil, mapReservationError(err)
	}

	return reservationResponse(reservation), nil
}

func mapReservationError(err error) error {
	switch {
	case errors.Is(err, domainerrors.ErrReservationNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domainerrors.ErrInsufficientStock):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, domainerrors.ErrReservationConflict),
		errors.Is(err, domainerrors.ErrIdempotencyConflict):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, domainerrors.ErrReservationExpired):
		return status.Error(codes.FailedPrecondition, err.Error())

	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

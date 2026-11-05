package usecases

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mouhamamd-kl/warehouse-service/internal/application/contracts/inputs"
	"github.com/mouhamamd-kl/warehouse-service/internal/application/ports/repositories"
	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
)

var (
	ErrInvalidReservation   = errors.New("invalid reservation")
	ErrInvalidReservationID = errors.New("reservation ID is required")
)

type ReservationUseCase struct {
	repository repositories.WarehouseRepository
}

func NewReservationUseCase(
	repository repositories.WarehouseRepository,
) *ReservationUseCase {
	return &ReservationUseCase{
		repository: repository,
	}
}

func (uc *ReservationUseCase) ReserveStock(
	ctx context.Context,
	input inputs.ReserveStockInput,
) (*entities.Reservation, error) {
	input.OrderID = strings.TrimSpace(input.OrderID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)

	if input.OrderID == "" ||
		input.IdempotencyKey == "" ||
		len(input.Items) == 0 ||
		len(input.Items) > 100 {
		return nil, ErrInvalidReservation
	}

	if input.ExpiresInSeconds < 0 ||
		input.ExpiresInSeconds > math.MaxInt64/int64(time.Second) {
		return nil, ErrInvalidReservation
	}

	quantities := make(map[int64]int64)

	for _, item := range input.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			return nil, ErrInvalidReservation
		}

		if quantities[item.ProductID] > math.MaxInt64-item.Quantity {
			return nil, ErrInvalidReservation
		}

		quantities[item.ProductID] += item.Quantity
	}

	items := make([]entities.ReservationItem, 0, len(quantities))

	for productID, quantity := range quantities {
		items = append(items, entities.ReservationItem{
			ProductID: productID,
			Quantity:  quantity,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID < items[j].ProductID
	})

	var expiresAt *time.Time

	if input.ExpiresInSeconds > 0 {
		expiration := time.Now().UTC().Add(
			time.Duration(input.ExpiresInSeconds) * time.Second,
		)
		expiresAt = &expiration
	}

	reservation := &entities.Reservation{
		OrderID:        input.OrderID,
		IdempotencyKey: input.IdempotencyKey,
		Status:         entities.ReservationActive,
		Items:          items,
		ExpiresAt:      expiresAt,
	}

	return uc.repository.ReserveStock(ctx, reservation)
}

func (uc *ReservationUseCase) GetReservation(
	ctx context.Context,
	reservationID string,
) (*entities.Reservation, error) {
	reservationID = strings.TrimSpace(reservationID)

	if reservationID == "" {
		return nil, ErrInvalidReservationID
	}

	return uc.repository.FindReservation(ctx, reservationID)
}

func (uc *ReservationUseCase) ReleaseReservation(
	ctx context.Context,
	reservationID string,
	reason string,
) (*entities.Reservation, error) {
	reservationID = strings.TrimSpace(reservationID)
	reason = strings.TrimSpace(reason)

	if reservationID == "" {
		return nil, ErrInvalidReservationID
	}

	if reason == "" {
		reason = "Reservation released"
	}

	return uc.repository.ReleaseReservation(
		ctx,
		reservationID,
		reason,
	)
}

func (uc *ReservationUseCase) ConfirmReservation(
	ctx context.Context,
	reservationID string,
) (*entities.Reservation, error) {
	reservationID = strings.TrimSpace(reservationID)

	if reservationID == "" {
		return nil, ErrInvalidReservationID
	}

	return uc.repository.ConfirmReservation(
		ctx,
		reservationID,
	)
}

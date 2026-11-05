package grpcclient

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	domainerrors "github.com/mouhamamd-kl/warehouse-service/internal/domain/errors"
)

// These decoders mirror the error mapping done on the server side (see the
// mapWarehouseError / mapReservationError helpers in the gRPC handlers).
// Unrecognized codes — Unavailable, DeadlineExceeded, transport errors — are
// returned untouched so callers keep the original status error.
//
// Note: a single wire code can stand for more than one domain sentinel
// (for example FailedPrecondition covers both insufficient stock and an
// expired reservation). The operation context selects the most likely one;
// resolving that ambiguity in a lossless way would require structured error
// details in the proto.

func decodeCreateInventoryError(err error) error {
	switch status.Code(err) {
	case codes.AlreadyExists:
		return domainerrors.ErrInventoryAlreadyExists
	default:
		return err
	}
}

func decodeGetInventoryError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return domainerrors.ErrInventoryNotFound
	default:
		return err
	}
}

func decodeAdjustStockError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return domainerrors.ErrInventoryNotFound
	case codes.AlreadyExists:
		return domainerrors.ErrIdempotencyConflict
	case codes.FailedPrecondition:
		return domainerrors.ErrInsufficientStock
	default:
		return err
	}
}

func decodeReserveStockError(err error) error {
	switch status.Code(err) {
	case codes.AlreadyExists:
		return domainerrors.ErrIdempotencyConflict
	case codes.FailedPrecondition:
		return domainerrors.ErrInsufficientStock
	default:
		return err
	}
}

func decodeFindReservationError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return domainerrors.ErrReservationNotFound
	default:
		return err
	}
}

func decodeReleaseConfirmError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return domainerrors.ErrReservationNotFound
	case codes.AlreadyExists:
		return domainerrors.ErrReservationConflict
	case codes.FailedPrecondition:
		return domainerrors.ErrReservationExpired
	default:
		return err
	}
}

package grpcclient

import (
	"fmt"
	"math"
	"time"

	warehousev1 "github.com/mouhamamd-kl/warehouse-service/gen/warehouse/v1"

	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
)

// The remote handler must encode timestamps with RFC3339 (see
// internal/infrastructure/grpc). These mappers mirror that contract.

func parseRFC3339(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"grpcclient: %q is not an RFC3339 timestamp: %w",
			field,
			err,
		)
	}

	return parsed, nil
}

func inventoryFromProto(
	response *warehousev1.InventoryResponse,
) (*entities.Inventory, error) {
	createdAt, err := parseRFC3339("created_at", response.GetCreatedAt())
	if err != nil {
		return nil, err
	}

	updatedAt, err := parseRFC3339("updated_at", response.GetUpdatedAt())
	if err != nil {
		return nil, err
	}

	return &entities.Inventory{
		ID:                response.GetId(),
		ProductID:         response.GetProductId(),
		QuantityOnHand:    response.GetQuantityOnHand(),
		QuantityReserved:  response.GetQuantityReserved(),
		LowStockThreshold: response.GetLowStockThreshold(),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func inventoryListFromProto(
	items []*warehousev1.InventoryResponse,
) ([]entities.Inventory, error) {
	result := make([]entities.Inventory, 0, len(items))

	for _, item := range items {
		inventory, err := inventoryFromProto(item)
		if err != nil {
			return nil, err
		}

		result = append(result, *inventory)
	}

	return result, nil
}

func movementListFromProto(
	items []*warehousev1.MovementResponse,
) ([]entities.InventoryMovement, error) {
	result := make([]entities.InventoryMovement, 0, len(items))

	for _, item := range items {
		createdAt, err := parseRFC3339("created_at", item.GetCreatedAt())
		if err != nil {
			return nil, err
		}

		result = append(result, entities.InventoryMovement{
			ID:             item.GetId(),
			InventoryID:    item.GetInventoryId(),
			ProductID:      item.GetProductId(),
			Type:           entities.MovementType(item.GetType()),
			Quantity:       item.GetQuantity(),
			QuantityBefore: item.GetQuantityBefore(),
			QuantityAfter:  item.GetQuantityAfter(),
			Reason:         item.GetReason(),
			ReferenceID:    item.GetReferenceId(),
			CreatedAt:      createdAt,
		})
	}

	return result, nil
}

func reservationFromProto(
	response *warehousev1.ReservationResponse,
) (*entities.Reservation, error) {
	createdAt, err := parseRFC3339("created_at", response.GetCreatedAt())
	if err != nil {
		return nil, err
	}

	var expiresAt *time.Time

	if value := response.GetExpiresAt(); value != "" {
		parsed, err := parseRFC3339("expires_at", value)
		if err != nil {
			return nil, err
		}

		expiresAt = &parsed
	}

	items := make([]entities.ReservationItem, 0, len(response.GetItems()))

	for _, item := range response.GetItems() {
		items = append(items, entities.ReservationItem{
			ProductID: item.GetProductId(),
			Quantity:  item.GetQuantity(),
		})
	}

	return &entities.Reservation{
		ID:        response.GetId(),
		OrderID:   response.GetOrderId(),
		Status:    entities.ReservationStatus(response.GetStatus()),
		Items:     items,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}, nil
}

// expiresInSeconds converts an absolute expiry from an incoming reservation
// into the expires_in_seconds the remote use case expects. The remote side
// adds it to its own "now", so a small clock/network skew is inherent to the
// wire contract; rounding up avoids lapses at second boundaries.
func expiresInSeconds(expiresAt *time.Time) int64 {
	if expiresAt == nil {
		return 0
	}

	seconds := int64(math.Ceil(time.Until(*expiresAt).Seconds()))

	if seconds < 0 {
		return 0
	}

	return seconds
}

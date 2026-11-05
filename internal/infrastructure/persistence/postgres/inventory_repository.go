package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
	domainerrors "github.com/mouhamamd-kl/warehouse-service/internal/domain/errors"
)

type InventoryRepository struct {
	db *pgxpool.Pool
}

func NewInventoryRepository(db *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{db: db}
}

func (r *InventoryRepository) FindByProductID(
	ctx context.Context,
	productID int64,
) (*entities.Inventory, error) {
	query := `
		SELECT
			id,
			product_id,
			quantity_on_hand,
			quantity_reserved,
			low_stock_threshold,
			created_at,
			updated_at
		FROM inventory
		WHERE product_id = $1
	`

	var inventory entities.Inventory

	err := r.db.QueryRow(ctx, query, productID).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.QuantityOnHand,
		&inventory.QuantityReserved,
		&inventory.LowStockThreshold,
		&inventory.CreatedAt,
		&inventory.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerrors.ErrInventoryNotFound
	}

	if err != nil {
		return nil, err
	}

	return &inventory, nil
}

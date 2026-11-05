package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
	domainerrors "github.com/mouhamamd-kl/warehouse-service/internal/domain/errors"
)

type WarehouseRepository struct {
	db *pgxpool.Pool
}

func NewWarehouseRepository(db *pgxpool.Pool) *WarehouseRepository {
	return &WarehouseRepository{db: db}
}

func (r *WarehouseRepository) CreateInventory(
	ctx context.Context,
	inventory *entities.Inventory,
) (*entities.Inventory, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO inventory (
			product_id,
			quantity_on_hand,
			quantity_reserved,
			low_stock_threshold
		)
		VALUES ($1, $2, 0, $3)
		RETURNING
			id, product_id, quantity_on_hand,
			quantity_reserved, low_stock_threshold,
			created_at, updated_at
	`,
		inventory.ProductID,
		inventory.QuantityOnHand,
		inventory.LowStockThreshold,
	).Scan(
		&inventory.ID,
		&inventory.ProductID,
		&inventory.QuantityOnHand,
		&inventory.QuantityReserved,
		&inventory.LowStockThreshold,
		&inventory.CreatedAt,
		&inventory.UpdatedAt,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return nil, domainerrors.ErrInventoryAlreadyExists
		}
		return nil, err
	}

	if inventory.QuantityOnHand > 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_movements (
				inventory_id,
				type,
				quantity,
				quantity_before,
				quantity_after,
				reason
			)
			VALUES ($1, 'stock_added', $2, 0, $2, 'Initial stock')
		`, inventory.ID, inventory.QuantityOnHand)

		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return inventory, nil
}

func (r *WarehouseRepository) FindByProductID(
	ctx context.Context,
	productID int64,
) (*entities.Inventory, error) {
	var inventory entities.Inventory

	err := r.db.QueryRow(ctx, `
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
	`, productID).Scan(
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

func (r *WarehouseRepository) FindAll(
	ctx context.Context,
	page, limit int,
) ([]entities.Inventory, int64, error) {
	return r.listInventory(ctx, page, limit, false)
}

func (r *WarehouseRepository) FindLowStock(
	ctx context.Context,
	page, limit int,
) ([]entities.Inventory, int64, error) {
	return r.listInventory(ctx, page, limit, true)
}

func (r *WarehouseRepository) listInventory(
	ctx context.Context,
	page, limit int,
	lowStockOnly bool,
) ([]entities.Inventory, int64, error) {
	offset := (page - 1) * limit

	filter := ""
	if lowStockOnly {
		filter = `
			WHERE quantity_on_hand - quantity_reserved
			<= low_stock_threshold
		`
	}

	var total int64

	countQuery := fmt.Sprintf(
		"SELECT COUNT(*) FROM inventory %s",
		filter,
	)

	if err := r.db.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT
			id,
			product_id,
			quantity_on_hand,
			quantity_reserved,
			low_stock_threshold,
			created_at,
			updated_at
		FROM inventory
		%s
		ORDER BY id
		LIMIT $1 OFFSET $2
	`, filter)

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]entities.Inventory, 0)

	for rows.Next() {
		var item entities.Inventory

		if err := rows.Scan(
			&item.ID,
			&item.ProductID,
			&item.QuantityOnHand,
			&item.QuantityReserved,
			&item.LowStockThreshold,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// AdjustStock uses a database idempotency key.
// A retry with the same key and same request returns the original result.
func (r *WarehouseRepository) AdjustStock(
	ctx context.Context,
	productID int64,
	quantity int64,
	operation string,
	reason string,
	referenceID string,
	idempotencyKey string,
) (*entities.Inventory, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Serialize requests sharing an idempotency key.
	if idempotencyKey != "" {
		_, err = tx.Exec(ctx, `
			SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
		`, idempotencyKey)
		if err != nil {
			return nil, err
		}

		var oldProductID, oldQuantity int64
		var oldOperation, oldReason, oldReference string

		err = tx.QueryRow(ctx, `
			SELECT
				i.product_id,
				m.quantity,
				m.type,
				m.reason,
				COALESCE(m.reference_id, '')
			FROM inventory_movements m
			JOIN inventory i ON i.id = m.inventory_id
			WHERE m.idempotency_key = $1
		`, idempotencyKey).Scan(
			&oldProductID,
			&oldQuantity,
			&oldOperation,
			&oldReason,
			&oldReference,
		)

		if err == nil {
			expectedType := "stock_added"
			if operation == "remove" {
				expectedType = "stock_removed"
			}

			if oldProductID != productID ||
				oldQuantity != quantity ||
				oldOperation != expectedType ||
				oldReason != reason ||
				oldReference != referenceID {
				return nil, domainerrors.ErrIdempotencyConflict
			}

			var inventory entities.Inventory

			err = tx.QueryRow(ctx, `
				SELECT
					id, product_id, quantity_on_hand,
					quantity_reserved, low_stock_threshold,
					created_at, updated_at
				FROM inventory
				WHERE product_id = $1
			`, productID).Scan(
				&inventory.ID,
				&inventory.ProductID,
				&inventory.QuantityOnHand,
				&inventory.QuantityReserved,
				&inventory.LowStockThreshold,
				&inventory.CreatedAt,
				&inventory.UpdatedAt,
			)

			if err != nil {
				return nil, err
			}

			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}

			return &inventory, nil
		}

		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	var inventory entities.Inventory

	err = tx.QueryRow(ctx, `
		SELECT
			id, product_id, quantity_on_hand,
			quantity_reserved, low_stock_threshold,
			created_at, updated_at
		FROM inventory
		WHERE product_id = $1
		FOR UPDATE
	`, productID).Scan(
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

	before := inventory.QuantityOnHand

	switch operation {
	case "add":
		if inventory.QuantityOnHand > int64(^uint64(0)>>1)-quantity {
			return nil, errors.New("stock quantity overflow")
		}
		inventory.QuantityOnHand += quantity

	case "remove":
		if inventory.QuantityOnHand-inventory.QuantityReserved < quantity {
			return nil, domainerrors.ErrInsufficientStock
		}
		inventory.QuantityOnHand -= quantity

	default:
		return nil, errors.New("invalid stock operation")
	}

	err = tx.QueryRow(ctx, `
		UPDATE inventory
		SET quantity_on_hand = $1,
		    updated_at = NOW()
		WHERE id = $2
		RETURNING updated_at
	`, inventory.QuantityOnHand, inventory.ID).Scan(&inventory.UpdatedAt)

	if err != nil {
		return nil, err
	}

	movementType := "stock_added"
	if operation == "remove" {
		movementType = "stock_removed"
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO inventory_movements (
			inventory_id,
			type,
			quantity,
			quantity_before,
			quantity_after,
			reason,
			reference_id,
			idempotency_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''))
	`,
		inventory.ID,
		movementType,
		quantity,
		before,
		inventory.QuantityOnHand,
		reason,
		referenceID,
		idempotencyKey,
	)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &inventory, nil
}

func (r *WarehouseRepository) GetMovements(
	ctx context.Context,
	productID int64,
	page, limit int,
) ([]entities.InventoryMovement, int64, error) {
	var inventoryID int64

	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM inventory
		WHERE product_id = $1
	`, productID).Scan(&inventoryID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, domainerrors.ErrInventoryNotFound
	}
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit

	var total int64

	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM inventory_movements
		WHERE inventory_id = $1
	`, inventoryID).Scan(&total)

	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			inventory_id,
			type,
			quantity,
			quantity_before,
			quantity_after,
			reason,
			COALESCE(reference_id, ''),
			created_at
		FROM inventory_movements
		WHERE inventory_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, inventoryID, limit, offset)

	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]entities.InventoryMovement, 0)

	for rows.Next() {
		var item entities.InventoryMovement

		if err := rows.Scan(
			&item.ID,
			&item.InventoryID,
			&item.Type,
			&item.Quantity,
			&item.QuantityBefore,
			&item.QuantityAfter,
			&item.Reason,
			&item.ReferenceID,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}

		item.ProductID = productID
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

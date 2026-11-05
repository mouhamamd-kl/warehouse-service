package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"sort"
	"time"

	"github.com/mouhamamd-kl/warehouse-service/internal/domain/entities"
	domainerrors "github.com/mouhamamd-kl/warehouse-service/internal/domain/errors"
)

type ReservationRepository struct {
	db *pgxpool.Pool
}

func NewReservationRepository(db *pgxpool.Pool) *ReservationRepository {
	return &ReservationRepository{db: db}
}

func (r *ReservationRepository) ReserveStock(
	ctx context.Context,
	reservation *entities.Reservation,
) (*entities.Reservation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Serialize requests using the same key.
	_, err = tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
	`, reservation.IdempotencyKey)
	if err != nil {
		return nil, err
	}

	existing, err := findReservationByKey(
		ctx,
		tx,
		reservation.IdempotencyKey,
	)

	if err == nil {
		if existing.OrderID != reservation.OrderID ||
			!sameReservationItems(existing.Items, reservation.Items) {
			return nil, domainerrors.ErrIdempotencyConflict
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return existing, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	reservation.ID, err = generateReservationID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	reservation.CreatedAt = now
	reservation.UpdatedAt = now

	if reservation.ExpiresAt != nil &&
		!reservation.ExpiresAt.After(now) {
		return nil, domainerrors.ErrReservationExpired
	}

	sort.Slice(reservation.Items, func(i, j int) bool {
		return reservation.Items[i].ProductID < reservation.Items[j].ProductID
	})

	type lockedInventory struct {
		ID               int64
		ProductID        int64
		QuantityOnHand   int64
		QuantityReserved int64
	}

	locked := make([]lockedInventory, 0, len(reservation.Items))

	// Acquire all inventory locks in sorted product ID order.
	for _, item := range reservation.Items {
		var inventory lockedInventory

		err := tx.QueryRow(ctx, `
			SELECT
				id,
				product_id,
				quantity_on_hand,
				quantity_reserved
			FROM inventory
			WHERE product_id = $1
			FOR UPDATE
		`, item.ProductID).Scan(
			&inventory.ID,
			&inventory.ProductID,
			&inventory.QuantityOnHand,
			&inventory.QuantityReserved,
		)

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerrors.ErrInventoryNotFound
		}
		if err != nil {
			return nil, err
		}

		if inventory.QuantityOnHand-inventory.QuantityReserved < item.Quantity {
			return nil, domainerrors.ErrInsufficientStock
		}

		locked = append(locked, inventory)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO reservations (
			id,
			order_id,
			idempotency_key,
			status,
			expires_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, 'active', $4, $5, $5)
	`,
		reservation.ID,
		reservation.OrderID,
		reservation.IdempotencyKey,
		reservation.ExpiresAt,
		now,
	)

	if err != nil {
		return nil, err
	}

	for index, item := range reservation.Items {
		inventory := locked[index]

		_, err = tx.Exec(ctx, `
			INSERT INTO reservation_items (
				reservation_id,
				product_id,
				quantity
			)
			VALUES ($1, $2, $3)
		`, reservation.ID, item.ProductID, item.Quantity)

		if err != nil {
			return nil, err
		}

		before := inventory.QuantityReserved
		after := before + item.Quantity

		_, err = tx.Exec(ctx, `
			UPDATE inventory
			SET quantity_reserved = $1,
			    updated_at = NOW()
			WHERE id = $2
		`, after, inventory.ID)

		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_movements (
				inventory_id,
				type,
				quantity,
				quantity_before,
				quantity_after,
				reason,
				reference_id
			)
			VALUES (
				$1,
				'reserved',
				$2,
				$3,
				$4,
				'Order reservation',
				$5
			)
		`,
			inventory.ID,
			item.Quantity,
			before,
			after,
			reservation.ID,
		)

		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.FindReservation(ctx, reservation.ID)
}

func (r *ReservationRepository) FindReservation(
	ctx context.Context,
	reservationID string,
) (*entities.Reservation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	reservation, err := r.lockReservation(ctx, tx, reservationID)
	if err != nil {
		return nil, err
	}

	if reservation.Status == entities.ReservationActive &&
		reservation.ExpiresAt != nil &&
		!reservation.ExpiresAt.After(time.Now().UTC()) {

		reservation, err = r.expireReservationTx(
			ctx,
			tx,
			reservation,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return reservation, nil
}

func (r *ReservationRepository) ReleaseReservation(
	ctx context.Context,
	reservationID string,
	reason string,
) (*entities.Reservation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	reservation, err := r.lockReservation(ctx, tx, reservationID)
	if err != nil {
		return nil, err
	}

	if reservation.Status == entities.ReservationReleased ||
		reservation.Status == entities.ReservationExpired {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return reservation, nil
	}

	if reservation.Status != entities.ReservationActive {
		return nil, domainerrors.ErrReservationConflict
	}

	if reservation.ExpiresAt != nil &&
		!reservation.ExpiresAt.After(time.Now().UTC()) {
		reservation, err = r.expireReservationTx(ctx, tx, reservation)
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return reservation, nil
	}

	if err := r.releaseItemsTx(ctx, tx, reservation, reason); err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE reservations
		SET status = 'released',
		    updated_at = NOW()
		WHERE id = $1
	`, reservationID)

	if err != nil {
		return nil, err
	}

	reservation.Status = entities.ReservationReleased

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.FindReservation(ctx, reservationID)
}

func (r *ReservationRepository) ConfirmReservation(
	ctx context.Context,
	reservationID string,
) (*entities.Reservation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	reservation, err := r.lockReservation(ctx, tx, reservationID)
	if err != nil {
		return nil, err
	}

	if reservation.Status == entities.ReservationConfirmed {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return reservation, nil
	}

	if reservation.Status != entities.ReservationActive {
		return nil, domainerrors.ErrReservationConflict
	}

	if reservation.ExpiresAt != nil &&
		!reservation.ExpiresAt.After(time.Now().UTC()) {
		_, err = r.expireReservationTx(ctx, tx, reservation)
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return nil, domainerrors.ErrReservationExpired
	}

	// The reservation items are already sorted by product ID.
	// Lock all inventory records before changing any balances.
	locked, err := lockReservationInventory(ctx, tx, reservation.Items)
	if err != nil {
		return nil, err
	}

	for index, item := range reservation.Items {
		inventory := locked[index]

		if inventory.QuantityOnHand < item.Quantity ||
			inventory.QuantityReserved < item.Quantity {
			return nil, domainerrors.ErrReservationBalanceInconsistent
		}

		newOnHand := inventory.QuantityOnHand - item.Quantity
		newReserved := inventory.QuantityReserved - item.Quantity

		_, err = tx.Exec(ctx, `
			UPDATE inventory
			SET quantity_on_hand = $1,
			    quantity_reserved = $2,
			    updated_at = NOW()
			WHERE id = $3
		`, newOnHand, newReserved, inventory.ID)

		if err != nil {
			return nil, err
		}

		// Physical stock movement.
		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_movements (
				inventory_id,
				type,
				quantity,
				quantity_before,
				quantity_after,
				reason,
				reference_id
			)
			VALUES ($1, 'sold', $2, $3, $4, 'Reservation confirmed', $5)
		`,
			inventory.ID,
			item.Quantity,
			inventory.QuantityOnHand,
			newOnHand,
			reservation.ID,
		)

		if err != nil {
			return nil, err
		}

		// Reserved balance movement.
		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_movements (
				inventory_id,
				type,
				quantity,
				quantity_before,
				quantity_after,
				reason,
				reference_id
			)
			VALUES ($1, 'released', $2, $3, $4, 'Reservation consumed', $5)
		`,
			inventory.ID,
			item.Quantity,
			inventory.QuantityReserved,
			newReserved,
			reservation.ID,
		)

		if err != nil {
			return nil, err
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE reservations
		SET status = 'confirmed',
		    updated_at = NOW()
		WHERE id = $1
	`, reservationID)

	if err != nil {
		return nil, err
	}

	reservation.Status = entities.ReservationConfirmed

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.FindReservation(ctx, reservationID)
}

// expireReservationTx releases reserved quantities and marks the reservation
// expired. The caller must hold the reservation row lock.
func (r *ReservationRepository) expireReservationTx(
	ctx context.Context,
	tx pgx.Tx,
	reservation *entities.Reservation,
) (*entities.Reservation, error) {
	if reservation.Status != entities.ReservationActive {
		return reservation, nil
	}

	if err := r.releaseItemsTx(
		ctx,
		tx,
		reservation,
		"Reservation expired",
	); err != nil {
		return nil, err
	}

	_, err := tx.Exec(ctx, `
		UPDATE reservations
		SET status = 'expired',
		    updated_at = NOW()
		WHERE id = $1
	`, reservation.ID)

	if err != nil {
		return nil, err
	}

	reservation.Status = entities.ReservationExpired
	reservation.UpdatedAt = time.Now().UTC()

	return reservation, nil
}

// releaseItemsTx locks inventory rows in sorted order and releases reserved
// balances. The caller must hold the reservation row lock.
func (r *ReservationRepository) releaseItemsTx(
	ctx context.Context,
	tx pgx.Tx,
	reservation *entities.Reservation,
	reason string,
) error {
	locked, err := lockReservationInventory(ctx, tx, reservation.Items)
	if err != nil {
		return err
	}

	for index, item := range reservation.Items {
		inventory := locked[index]

		if inventory.QuantityReserved < item.Quantity {
			return domainerrors.ErrReservationBalanceInconsistent
		}

		after := inventory.QuantityReserved - item.Quantity

		_, err = tx.Exec(ctx, `
			UPDATE inventory
			SET quantity_reserved = $1,
			    updated_at = NOW()
			WHERE id = $2
		`, after, inventory.ID)

		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_movements (
				inventory_id,
				type,
				quantity,
				quantity_before,
				quantity_after,
				reason,
				reference_id
			)
			VALUES ($1, 'released', $2, $3, $4, $5, $6)
		`,
			inventory.ID,
			item.Quantity,
			inventory.QuantityReserved,
			after,
			reason,
			reservation.ID,
		)

		if err != nil {
			return err
		}
	}

	return nil
}

type reservationInventory struct {
	ID               int64
	ProductID        int64
	QuantityOnHand   int64
	QuantityReserved int64
}

func lockReservationInventory(
	ctx context.Context,
	tx pgx.Tx,
	items []entities.ReservationItem,
) ([]reservationInventory, error) {
	sortedItems := append([]entities.ReservationItem(nil), items...)

	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductID < sortedItems[j].ProductID
	})

	locked := make([]reservationInventory, 0, len(sortedItems))

	for _, item := range sortedItems {
		var inventory reservationInventory

		err := tx.QueryRow(ctx, `
			SELECT
				id,
				product_id,
				quantity_on_hand,
				quantity_reserved
			FROM inventory
			WHERE product_id = $1
			FOR UPDATE
		`, item.ProductID).Scan(
			&inventory.ID,
			&inventory.ProductID,
			&inventory.QuantityOnHand,
			&inventory.QuantityReserved,
		)

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerrors.ErrInventoryNotFound
		}

		if err != nil {
			return nil, err
		}

		locked = append(locked, inventory)
	}

	return locked, nil
}

func (r *ReservationRepository) lockReservation(
	ctx context.Context,
	tx pgx.Tx,
	reservationID string,
) (*entities.Reservation, error) {
	var reservation entities.Reservation

	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			order_id,
			idempotency_key,
			status,
			expires_at,
			created_at,
			updated_at
		FROM reservations
		WHERE id = $1
		FOR UPDATE
	`, reservationID).Scan(
		&reservation.ID,
		&reservation.OrderID,
		&reservation.IdempotencyKey,
		&reservation.Status,
		&reservation.ExpiresAt,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerrors.ErrReservationNotFound
	}

	if err != nil {
		return nil, err
	}

	items, err := loadReservationItems(ctx, tx, reservationID)
	if err != nil {
		return nil, err
	}

	reservation.Items = items

	return &reservation, nil
}

func (r *ReservationRepository) findReservationTx(
	ctx context.Context,
	tx pgx.Tx,
	reservationID string,
) (*entities.Reservation, error) {
	var reservation entities.Reservation

	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			order_id,
			idempotency_key,
			status,
			expires_at,
			created_at,
			updated_at
		FROM reservations
		WHERE id = $1
	`, reservationID).Scan(
		&reservation.ID,
		&reservation.OrderID,
		&reservation.IdempotencyKey,
		&reservation.Status,
		&reservation.ExpiresAt,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerrors.ErrReservationNotFound
	}

	if err != nil {
		return nil, err
	}

	reservation.Items, err = loadReservationItems(ctx, tx, reservationID)
	if err != nil {
		return nil, err
	}

	return &reservation, nil
}

func findReservationByKey(
	ctx context.Context,
	tx pgx.Tx,
	key string,
) (*entities.Reservation, error) {
	var reservation entities.Reservation

	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			order_id,
			idempotency_key,
			status,
			expires_at,
			created_at,
			updated_at
		FROM reservations
		WHERE idempotency_key = $1
	`, key).Scan(
		&reservation.ID,
		&reservation.OrderID,
		&reservation.IdempotencyKey,
		&reservation.Status,
		&reservation.ExpiresAt,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	reservation.Items, err = loadReservationItems(ctx, tx, reservation.ID)
	if err != nil {
		return nil, err
	}

	return &reservation, nil
}

func loadReservationItems(
	ctx context.Context,
	tx pgx.Tx,
	reservationID string,
) ([]entities.ReservationItem, error) {
	rows, err := tx.Query(ctx, `
		SELECT product_id, quantity
		FROM reservation_items
		WHERE reservation_id = $1
		ORDER BY product_id
	`, reservationID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entities.ReservationItem, 0)

	for rows.Next() {
		var item entities.ReservationItem

		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func sameReservationItems(
	a, b []entities.ReservationItem,
) bool {
	if len(a) != len(b) {
		return false
	}

	left := append([]entities.ReservationItem(nil), a...)
	right := append([]entities.ReservationItem(nil), b...)

	sort.Slice(left, func(i, j int) bool {
		return left[i].ProductID < left[j].ProductID
	})

	sort.Slice(right, func(i, j int) bool {
		return right[i].ProductID < right[j].ProductID
	})

	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}

func generateReservationID() (string, error) {
	var b [16]byte

	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}

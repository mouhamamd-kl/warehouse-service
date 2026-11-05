
-- +goose Up

ALTER TABLE inventory_movements
ADD COLUMN idempotency_key VARCHAR(255);

CREATE UNIQUE INDEX uq_inventory_movements_idempotency
ON inventory_movements (idempotency_key)
WHERE idempotency_key IS NOT NULL;


-- +goose Down

DROP INDEX IF EXISTS uq_inventory_movements_idempotency;

ALTER TABLE inventory_movements
DROP COLUMN IF EXISTS idempotency_key;

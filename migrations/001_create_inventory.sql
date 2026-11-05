
-- +goose Up

CREATE TABLE inventory (
    id BIGSERIAL PRIMARY KEY,

    product_id BIGINT NOT NULL UNIQUE,

    quantity_on_hand BIGINT NOT NULL DEFAULT 0,
    quantity_reserved BIGINT NOT NULL DEFAULT 0,
    low_stock_threshold BIGINT NOT NULL DEFAULT 5,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_inventory_product_id
        CHECK (product_id > 0),

    CONSTRAINT chk_inventory_quantity_on_hand
        CHECK (quantity_on_hand >= 0),

    CONSTRAINT chk_inventory_quantity_reserved
        CHECK (
            quantity_reserved >= 0
            AND quantity_reserved <= quantity_on_hand
        ),

    CONSTRAINT chk_inventory_low_stock_threshold
        CHECK (low_stock_threshold >= 0)
);

CREATE INDEX idx_inventory_low_stock
ON inventory ((quantity_on_hand - quantity_reserved));

-- +goose Down

DROP TABLE IF EXISTS inventory;

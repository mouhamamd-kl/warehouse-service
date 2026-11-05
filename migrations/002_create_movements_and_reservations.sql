
-- +goose Up

CREATE TABLE inventory_movements (
    id BIGSERIAL PRIMARY KEY,
    inventory_id BIGINT NOT NULL REFERENCES inventory(id),
    type VARCHAR(30) NOT NULL,
    quantity BIGINT NOT NULL,
    quantity_before BIGINT NOT NULL,
    quantity_after BIGINT NOT NULL,
    reason TEXT NOT NULL,
    reference_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_movement_quantity CHECK (quantity > 0),
    CONSTRAINT chk_movement_type CHECK (
        type IN (
            'stock_added',
            'stock_removed',
            'reserved',
            'released',
            'sold'
        )
    )
);

CREATE INDEX idx_movements_inventory_created
ON inventory_movements (inventory_id, created_at DESC);


CREATE TABLE reservations (
    id UUID PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_reservation_status CHECK (
        status IN ('active', 'released', 'confirmed', 'expired')
    )
);

CREATE INDEX idx_reservations_order_id
ON reservations (order_id);


CREATE TABLE reservation_items (
    id BIGSERIAL PRIMARY KEY,
    reservation_id UUID NOT NULL
        REFERENCES reservations(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL,
    quantity BIGINT NOT NULL,

    CONSTRAINT chk_reservation_item_quantity CHECK (quantity > 0),
    CONSTRAINT uq_reservation_product UNIQUE (reservation_id, product_id)
);

CREATE INDEX idx_reservation_items_product
ON reservation_items (product_id);


-- +goose Down

DROP TABLE IF EXISTS reservation_items;
DROP TABLE IF EXISTS reservations;
DROP TABLE IF EXISTS inventory_movements;

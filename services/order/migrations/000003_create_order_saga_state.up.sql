CREATE TABLE order_saga_state (
    order_id             UUID PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    payment_status       VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                                      CHECK (payment_status IN ('PENDING', 'COMPLETED', 'FAILED')),
    inventory_status     VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                                      CHECK (inventory_status IN ('PENDING', 'RESERVED', 'FAILED')),
    compensation_status  VARCHAR(20),
    last_event_type      VARCHAR(50),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

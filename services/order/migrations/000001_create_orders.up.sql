CREATE TABLE orders (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL,
    status           VARCHAR(30) NOT NULL DEFAULT 'CREATED'
                                    CHECK (status IN (
                                        'CREATED',
                                        'PAYMENT_PENDING',
                                        'PAYMENT_COMPLETED',
                                        'INVENTORY_RESERVING',
                                        'COMPLETED',
                                        'CANCELLED',
                                        'COMPENSATING'
                                    )),
    total_amount     BIGINT NOT NULL CHECK (total_amount >= 0),
    currency         CHAR(3) NOT NULL DEFAULT 'JPY',
    failure_reason   TEXT,
    idempotency_key  VARCHAR(255) UNIQUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

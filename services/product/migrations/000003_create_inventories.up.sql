CREATE TABLE inventories (
    product_id     UUID    PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    stock          INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
    reserved_stock INTEGER NOT NULL DEFAULT 0 CHECK (reserved_stock >= 0),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

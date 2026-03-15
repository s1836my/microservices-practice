CREATE TABLE products (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id   UUID         NOT NULL,
    category_id UUID         NOT NULL REFERENCES categories(id),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    price       BIGINT       NOT NULL CHECK (price >= 0),
    status      VARCHAR(20)  NOT NULL DEFAULT 'active'
                             CHECK (status IN ('draft', 'active', 'inactive', 'deleted')),
    images      JSONB        NOT NULL DEFAULT '[]',
    attributes  JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_seller_id   ON products(seller_id);
CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_status      ON products(status);
CREATE INDEX idx_products_price       ON products(price);
CREATE INDEX idx_products_created_at  ON products(created_at DESC);

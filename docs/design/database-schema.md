# MicroMart — データベーススキーマ設計

> **最終更新**: 2026-03-12
> **ステータス**: 設計フェーズ

各サービスは **Database per Service** パターンに従い、独立した PostgreSQL データベースを所有する。他サービスのテーブルへの直接アクセスは禁止。

---

## 1. User Service DB (`micromart_users`)

### users

```sql
CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name          VARCHAR(100) NOT NULL,
    role          VARCHAR(20)  NOT NULL DEFAULT 'customer'
                                CHECK (role IN ('customer', 'seller', 'admin')),
    status        VARCHAR(20)  NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
```

### seller_profiles

```sql
CREATE TABLE seller_profiles (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    shop_name   VARCHAR(100) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

### refresh_tokens

```sql
CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ  NOT NULL,
    revoked    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
```

---

## 2. Product Service DB (`micromart_products`)

### categories

```sql
CREATE TABLE categories (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    slug        VARCHAR(100) NOT NULL UNIQUE,
    parent_id   UUID        REFERENCES categories(id) ON DELETE SET NULL,
    sort_order  INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_slug ON categories(slug);
CREATE INDEX idx_categories_parent_id ON categories(parent_id);
```

### products

```sql
CREATE TABLE products (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id    UUID          NOT NULL,   -- User Service の user_id (外部参照なし)
    category_id  UUID          NOT NULL REFERENCES categories(id),
    name         VARCHAR(255)  NOT NULL,
    description  TEXT,
    price        BIGINT        NOT NULL CHECK (price >= 0),  -- 円（銭以下切り捨て）
    status       VARCHAR(20)   NOT NULL DEFAULT 'active'
                                CHECK (status IN ('draft', 'active', 'inactive', 'deleted')),
    images       JSONB         NOT NULL DEFAULT '[]',
    attributes   JSONB         NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_seller_id   ON products(seller_id);
CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_status      ON products(status);
CREATE INDEX idx_products_price       ON products(price);
CREATE INDEX idx_products_created_at  ON products(created_at DESC);
```

### inventories

```sql
-- products と 1:1 (在庫管理を分離してロック競合を最小化)
CREATE TABLE inventories (
    product_id       UUID    PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    stock            INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
    reserved_stock   INTEGER NOT NULL DEFAULT 0 CHECK (reserved_stock >= 0),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 在庫の楽観的ロック更新例
-- UPDATE inventories
--   SET stock = stock - $1, updated_at = NOW()
-- WHERE product_id = $2 AND stock >= $1
```

### product_outbox

```sql
-- Transactional Outbox パターン: DBトランザクション内でイベントを記録
CREATE TABLE product_outbox (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(50)  NOT NULL,
    payload      JSONB        NOT NULL,
    published    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_product_outbox_unpublished ON product_outbox(created_at) WHERE published = FALSE;
```

---

## 3. Order Service DB (`micromart_orders`)

### orders

```sql
CREATE TABLE orders (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID          NOT NULL,   -- User Service の user_id
    status           VARCHAR(30)   NOT NULL DEFAULT 'CREATED'
                                    CHECK (status IN (
                                        'CREATED',
                                        'PAYMENT_PENDING',
                                        'PAYMENT_COMPLETED',
                                        'INVENTORY_RESERVING',
                                        'COMPLETED',
                                        'CANCELLED',
                                        'COMPENSATING'
                                    )),
    total_amount     BIGINT        NOT NULL CHECK (total_amount >= 0),
    failure_reason   TEXT,
    idempotency_key  VARCHAR(255)  UNIQUE,    -- 二重注文防止
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_id    ON orders(user_id);
CREATE INDEX idx_orders_status     ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);
```

### order_items

```sql
CREATE TABLE order_items (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID    NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id  UUID    NOT NULL,   -- Product Service の product_id
    seller_id   UUID    NOT NULL,
    product_name VARCHAR(255) NOT NULL,  -- 注文時点の商品名をスナップショット
    unit_price  BIGINT  NOT NULL CHECK (unit_price >= 0),
    quantity    INTEGER NOT NULL CHECK (quantity > 0),
    subtotal    BIGINT  GENERATED ALWAYS AS (unit_price * quantity) STORED
);

CREATE INDEX idx_order_items_order_id   ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);
```

### order_saga_state

```sql
-- Saga 状態追跡テーブル
CREATE TABLE order_saga_state (
    order_id              UUID        PRIMARY KEY REFERENCES orders(id),
    payment_status        VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                                       CHECK (payment_status IN ('PENDING', 'COMPLETED', 'FAILED')),
    inventory_status      VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                                       CHECK (inventory_status IN ('PENDING', 'RESERVED', 'FAILED')),
    compensation_status   VARCHAR(20),
    last_event_type       VARCHAR(50),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### order_outbox

```sql
CREATE TABLE order_outbox (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(50)  NOT NULL,
    payload      JSONB        NOT NULL,
    published    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_order_outbox_unpublished ON order_outbox(created_at) WHERE published = FALSE;
```

---

## 4. Payment Service DB (`micromart_payments`)

### payments

```sql
CREATE TABLE payments (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID          NOT NULL UNIQUE,  -- 注文1件につき1決済
    user_id          UUID          NOT NULL,
    amount           BIGINT        NOT NULL CHECK (amount > 0),
    currency         CHAR(3)       NOT NULL DEFAULT 'JPY',
    status           VARCHAR(20)   NOT NULL DEFAULT 'PENDING'
                                    CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED', 'REFUNDED')),
    idempotency_key  VARCHAR(255)  NOT NULL UNIQUE,
    provider_txn_id  VARCHAR(255),  -- 外部決済プロバイダのトランザクションID（模擬）
    failure_reason   TEXT,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order_id        ON payments(order_id);
CREATE INDEX idx_payments_user_id         ON payments(user_id);
CREATE INDEX idx_payments_idempotency_key ON payments(idempotency_key);
```

### payment_outbox

```sql
CREATE TABLE payment_outbox (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(50)  NOT NULL,
    payload      JSONB        NOT NULL,
    published    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_payment_outbox_unpublished ON payment_outbox(created_at) WHERE published = FALSE;
```

---

## 5. マイグレーション管理 (golang-migrate)

```
services/{service}/migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_refresh_tokens.up.sql
├── 000002_create_refresh_tokens.down.sql
└── ...
```

実行コマンド（Makefile より）:
```makefile
# services/{service}/migrations/ 配下を適用
migrate-up:
    migrate -path ./migrations -database "$$DATABASE_URL" up

migrate-down:
    migrate -path ./migrations -database "$$DATABASE_URL" down 1
```

---

## 6. ER 図（概念）

```
[users] 1──────────────────────────── 0..1 [seller_profiles]
   │
   │ (user_id: 外部キー制約なし、サービス間はAPIのみ)
   │
[products] 1──── 1 [inventories]
    └── N [order_items] (スナップショット保存)

[orders] 1──── N [order_items]
[orders] 1──── 1 [order_saga_state]
[orders] 1──── 1 [payments]
```

---

## 7. インデックス・パフォーマンス指針

| テーブル | ホットクエリ | 推奨インデックス |
|---|---|---|
| `users` | email でログイン | `idx_users_email` (UNIQUE) |
| `products` | カテゴリ + ステータス + 価格帯 | 複合インデックス `(category_id, status, price)` |
| `orders` | user_id + 作成日時降順 | `(user_id, created_at DESC)` |
| `order_items` | order_id | `idx_order_items_order_id` |
| `payments` | idempotency_key 重複チェック | `idx_payments_idempotency_key` (UNIQUE) |
| `*_outbox` | 未送信レコード取得 | Partial Index `WHERE published = FALSE` |

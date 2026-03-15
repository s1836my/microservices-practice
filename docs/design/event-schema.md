# MicroMart — Kafka イベントスキーマ設計

> **最終更新**: 2026-03-12
> **ステータス**: 設計フェーズ

---

## 1. 設計方針

| 項目 | 方針 |
|---|---|
| **フォーマット** | JSON (将来的に Avro 移行可能な設計) |
| **ライブラリ** | segmentio/kafka-go |
| **パーティションキー** | 各イベントの主エンティティID (例: order_id, product_id) |
| **べき等性** | Consumer 側で `event_id` による重複チェック |
| **リトライ** | Producer: 最大3回 / Consumer: 処理失敗時 DLQ へ転送 |
| **スキーマ進化** | 後方互換性を保つ（フィールド追加は OK, 削除・型変更は禁止） |

---

## 2. 共通エンベロープ

全イベントに共通するエンベロープフィールド:

```json
{
  "event_id":      "uuid-v4",          // 重複排除キー
  "event_type":    "order.created",    // 型識別子 (詳細は §3 参照)
  "event_version": "1.0",              // スキーマバージョン
  "source":        "order-service",    // 発行元サービス
  "timestamp":     "2026-03-12T10:00:00Z",
  "correlation_id": "uuid-v4",         // トレース用 (OpenTelemetry trace_id)
  "payload": { ... }                   // イベント固有データ
}
```

---

## 3. トピック一覧

| トピック名 | 発行元 | 購読者 | 用途 |
|---|---|---|---|
| `product.events` | Product Service | Search Service | CQRS: Elasticsearch インデックス更新 |
| `order.created` | Order Service | Payment Service | Saga: 決済開始トリガー |
| `payment.completed` | Payment Service | Order Service | Saga: 決済成功通知 |
| `payment.failed` | Payment Service | Order Service | Saga: 決済失敗通知 → 注文キャンセル |
| `inventory.reserve` | Order Service | Product Service | Saga: 在庫確保依頼 |
| `inventory.reserved` | Product Service | Order Service | Saga: 在庫確保成功 |
| `inventory.reservation_failed` | Product Service | Order Service | Saga: 在庫不足通知 |
| `order.completed` | Order Service | Notification Service | 注文完了通知 |
| `order.cancelled` | Order Service | Notification Service, Product Service | 注文キャンセル通知 (在庫補償) |

---

## 4. イベントスキーマ定義

### 4.1 `product.events`

#### `product.created`

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "product.created",
  "event_version": "1.0",
  "source": "product-service",
  "timestamp": "2026-03-12T10:00:00Z",
  "correlation_id": "abc123",
  "payload": {
    "product_id":   "prod-uuid",
    "seller_id":    "seller-uuid",
    "category_id":  "cat-uuid",
    "name":         "ワイヤレスイヤホン Pro",
    "description":  "高音質ノイズキャンセリング対応",
    "price":        19800,
    "stock":        100,
    "status":       "active",
    "images":       ["https://cdn.example.com/img/prod-uuid/1.jpg"],
    "attributes":   {"color": "black", "weight_g": 56}
  }
}
```

#### `product.updated`

```json
{
  "event_type": "product.updated",
  "payload": {
    "product_id":  "prod-uuid",
    "changed_fields": {
      "price":     15800,
      "stock":     80,
      "updated_at": "2026-03-12T12:00:00Z"
    }
  }
}
```

#### `product.deleted`

```json
{
  "event_type": "product.deleted",
  "payload": {
    "product_id": "prod-uuid",
    "deleted_at": "2026-03-12T15:00:00Z"
  }
}
```

---

### 4.2 `order.created`

Order Service → Payment Service (Saga Step 1)

```json
{
  "event_id": "evt-uuid",
  "event_type": "order.created",
  "event_version": "1.0",
  "source": "order-service",
  "timestamp": "2026-03-12T10:00:00Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id":        "order-uuid",
    "user_id":         "user-uuid",
    "idempotency_key": "idem-key-uuid",
    "total_amount":    39600,
    "currency":        "JPY",
    "items": [
      {
        "product_id":   "prod-uuid",
        "product_name": "ワイヤレスイヤホン Pro",
        "seller_id":    "seller-uuid",
        "unit_price":   19800,
        "quantity":     2
      }
    ]
  }
}
```

---

### 4.3 `payment.completed`

Payment Service → Order Service (Saga Step 2 - 成功)

```json
{
  "event_id": "evt-uuid",
  "event_type": "payment.completed",
  "event_version": "1.0",
  "source": "payment-service",
  "timestamp": "2026-03-12T10:00:05Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id":        "order-uuid",
    "payment_id":      "payment-uuid",
    "amount":          39600,
    "currency":        "JPY",
    "provider_txn_id": "mock-txn-12345"
  }
}
```

---

### 4.4 `payment.failed`

Payment Service → Order Service (Saga Step 2 - 失敗)

```json
{
  "event_id": "evt-uuid",
  "event_type": "payment.failed",
  "event_version": "1.0",
  "source": "payment-service",
  "timestamp": "2026-03-12T10:00:05Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id":       "order-uuid",
    "payment_id":     "payment-uuid",
    "reason":         "insufficient_funds",
    "error_code":     "PAYMENT_DECLINED"
  }
}
```

---

### 4.5 `inventory.reserve`

Order Service → Product Service (Saga Step 3)

```json
{
  "event_id": "evt-uuid",
  "event_type": "inventory.reserve",
  "event_version": "1.0",
  "source": "order-service",
  "timestamp": "2026-03-12T10:00:06Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id": "order-uuid",
    "items": [
      {
        "product_id": "prod-uuid",
        "quantity":   2
      }
    ]
  }
}
```

---

### 4.6 `inventory.reserved`

Product Service → Order Service (Saga Step 3 - 成功)

```json
{
  "event_id": "evt-uuid",
  "event_type": "inventory.reserved",
  "event_version": "1.0",
  "source": "product-service",
  "timestamp": "2026-03-12T10:00:07Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id": "order-uuid",
    "reserved_items": [
      {
        "product_id":      "prod-uuid",
        "quantity":        2,
        "remaining_stock": 98
      }
    ]
  }
}
```

---

### 4.7 `inventory.reservation_failed`

Product Service → Order Service (Saga Step 3 - 失敗)

```json
{
  "event_id": "evt-uuid",
  "event_type": "inventory.reservation_failed",
  "event_version": "1.0",
  "source": "product-service",
  "timestamp": "2026-03-12T10:00:07Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id":   "order-uuid",
    "reason":     "insufficient_stock",
    "failed_items": [
      {
        "product_id":       "prod-uuid",
        "requested":        2,
        "available_stock":  1
      }
    ]
  }
}
```

---

### 4.8 `order.completed`

Order Service → Notification Service

```json
{
  "event_id": "evt-uuid",
  "event_type": "order.completed",
  "event_version": "1.0",
  "source": "order-service",
  "timestamp": "2026-03-12T10:00:08Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id":     "order-uuid",
    "user_id":      "user-uuid",
    "total_amount": 39600,
    "currency":     "JPY",
    "items": [
      {
        "product_name": "ワイヤレスイヤホン Pro",
        "quantity":     2,
        "unit_price":   19800
      }
    ],
    "completed_at": "2026-03-12T10:00:08Z"
  }
}
```

---

### 4.9 `order.cancelled`

Order Service → Notification Service, Product Service (補償トランザクション)

```json
{
  "event_id": "evt-uuid",
  "event_type": "order.cancelled",
  "event_version": "1.0",
  "source": "order-service",
  "timestamp": "2026-03-12T10:00:09Z",
  "correlation_id": "trace-uuid",
  "payload": {
    "order_id":       "order-uuid",
    "user_id":        "user-uuid",
    "reason":         "payment_failed",
    "cancel_stage":   "PAYMENT_PENDING",
    "items": [
      {
        "product_id": "prod-uuid",
        "quantity":   2
      }
    ],
    "cancelled_at": "2026-03-12T10:00:09Z"
  }
}
```

---

## 5. Saga フロー全体シーケンス

### 5.1 正常フロー

```
Client          API GW       Order Svc      Payment Svc    Product Svc   Notification
  │               │              │                │               │             │
  │─POST /orders─►│              │                │               │             │
  │               │─gRPC─────── ►│                │               │             │
  │               │              │─INSERT orders──►│              │             │
  │               │              │─INSERT outbox───►              │             │
  │               │◄─200 ────────│                │               │             │
  │◄─202 Accepted─│              │                │               │             │
  │               │              │                │               │             │
  │               │              │──order.created─────────────── ►│             │
  │               │              │                │─process payment│             │
  │               │              │                │─INSERT payments│             │
  │               │              │◄──payment.completed────────────│             │
  │               │              │                │               │             │
  │               │              │──inventory.reserve────────────────────────── ►│
  │               │              │                │               │─UPDATE stock│
  │               │              │◄──inventory.reserved──────────────────────── │
  │               │              │                │               │             │
  │               │              │─UPDATE orders (COMPLETED)      │             │
  │               │              │──order.completed ──────────────────────────────────►│
  │               │              │                │               │             │─email│
```

### 5.2 補償フロー (決済失敗時)

```
Order Svc      Payment Svc
    │               │
    │──order.created─►│
    │                 │── 決済失敗
    │◄─payment.failed─│
    │
    │─UPDATE orders (CANCELLED)
    │──order.cancelled ──► Notification (キャンセルメール)
```

---

## 6. Dead Letter Queue (DLQ)

処理失敗イベントは専用 DLQ トピックへ転送:

| 元トピック | DLQ トピック |
|---|---|
| `order.created` | `order.created.dlq` |
| `payment.completed` | `payment.completed.dlq` |
| `inventory.reserve` | `inventory.reserve.dlq` |

DLQ 消費は監視アラートと合わせて手動調査・リトライを行う。

---

## 7. Transactional Outbox パターン

データ不整合を防ぐため、DB 更新と Kafka 発行を同一トランザクション内に収める:

```go
// Order Service: 注文作成 + Outbox レコードを同一トランザクションで保存
func (s *orderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*model.Order, error) {
    return s.db.WithTransaction(ctx, func(tx *sql.Tx) error {
        // 1. orders テーブルに INSERT
        order, err := s.orderRepo.CreateWithTx(ctx, tx, req)
        if err != nil {
            return err
        }

        // 2. Outbox テーブルにイベントを記録
        event := buildOrderCreatedEvent(order)
        return s.outboxRepo.InsertWithTx(ctx, tx, event)
    })
}

// 別 goroutine の Outbox Poller が定期的に未送信レコードを Kafka へ発行
func (p *OutboxPoller) Run(ctx context.Context) {
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            p.publishPending(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

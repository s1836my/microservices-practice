package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/order/internal/model"
)

type pgOrderRepository struct {
	pool *pgxpool.Pool
}

type orderEventItemPayload struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name,omitempty"`
	SellerID    string `json:"seller_id,omitempty"`
	UnitPrice   int64  `json:"unit_price,omitempty"`
	Quantity    int32  `json:"quantity"`
}

type orderCreatedPayload struct {
	OrderID        string                  `json:"order_id"`
	UserID         string                  `json:"user_id"`
	IdempotencyKey string                  `json:"idempotency_key"`
	TotalAmount    int64                   `json:"total_amount"`
	Currency       string                  `json:"currency"`
	Items          []orderEventItemPayload `json:"items"`
}

type orderCancelledPayload struct {
	OrderID     string                  `json:"order_id"`
	UserID      string                  `json:"user_id"`
	Reason      string                  `json:"reason"`
	CancelStage string                  `json:"cancel_stage"`
	Items       []orderEventItemPayload `json:"items"`
	CancelledAt string                  `json:"cancelled_at"`
}

type eventEnvelope struct {
	EventID      string          `json:"event_id"`
	EventType    string          `json:"event_type"`
	EventVersion string          `json:"event_version"`
	Source       string          `json:"source"`
	Timestamp    string          `json:"timestamp"`
	Payload      json.RawMessage `json:"payload"`
}

func NewOrderRepository(pool *pgxpool.Pool) OrderRepository {
	return &pgOrderRepository{pool: pool}
}

func BuildOrderCreatedOutbox(order *model.Order) (*model.OutboxEvent, error) {
	items := make([]orderEventItemPayload, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, orderEventItemPayload{
			ProductID:   item.ProductID.String(),
			ProductName: item.ProductName,
			SellerID:    item.SellerID.String(),
			UnitPrice:   item.UnitPrice,
			Quantity:    item.Quantity,
		})
	}

	inner, err := json.Marshal(orderCreatedPayload{
		OrderID:        order.ID.String(),
		UserID:         order.UserID.String(),
		IdempotencyKey: order.IdempotencyKey,
		TotalAmount:    order.TotalAmount,
		Currency:       order.Currency,
		Items:          items,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal order.created payload: %w", err)
	}

	return buildOutboxEvent("order.created", inner)
}

func BuildOrderCancelledOutbox(order *model.Order, reason string) (*model.OutboxEvent, error) {
	items := make([]orderEventItemPayload, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, orderEventItemPayload{
			ProductID: item.ProductID.String(),
			Quantity:  item.Quantity,
		})
	}

	inner, err := json.Marshal(orderCancelledPayload{
		OrderID:     order.ID.String(),
		UserID:      order.UserID.String(),
		Reason:      reason,
		CancelStage: string(order.Status),
		Items:       items,
		CancelledAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal order.cancelled payload: %w", err)
	}

	return buildOutboxEvent("order.cancelled", inner)
}

func buildOutboxEvent(eventType string, payload json.RawMessage) (*model.OutboxEvent, error) {
	envelope, err := json.Marshal(eventEnvelope{
		EventID:      uuid.New().String(),
		EventType:    eventType,
		EventVersion: "1.0",
		Source:       "order-service",
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Payload:      payload,
	})
	if err != nil {
		return nil, err
	}
	return &model.OutboxEvent{
		ID:        uuid.New(),
		EventType: eventType,
		Payload:   envelope,
	}, nil
}

func (r *pgOrderRepository) Create(ctx context.Context, order *model.Order, saga *model.SagaState, outbox *model.OutboxEvent) (*model.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const insertOrder = `
		INSERT INTO orders (id, user_id, status, total_amount, currency, failure_reason, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, insertOrder,
		order.ID, order.UserID, string(order.Status), order.TotalAmount, order.Currency, order.FailureReason, order.IdempotencyKey,
	).Scan(&createdAt, &updatedAt)
	if err != nil {
		return nil, mapWriteError(err, "insert order")
	}

	const insertItem = `
		INSERT INTO order_items (id, order_id, product_id, seller_id, product_name, unit_price, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING subtotal
	`
	items := make([]model.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		var subtotal int64
		err = tx.QueryRow(ctx, insertItem,
			item.ID, order.ID, item.ProductID, item.SellerID, item.ProductName, item.UnitPrice, item.Quantity,
		).Scan(&subtotal)
		if err != nil {
			return nil, mapWriteError(err, "insert order item")
		}
		item.OrderID = order.ID
		item.Subtotal = subtotal
		items = append(items, item)
	}

	const insertSaga = `
		INSERT INTO order_saga_state (order_id, payment_status, inventory_status, compensation_status, last_event_type)
		VALUES ($1, $2, $3, $4, $5)
	`
	if _, err = tx.Exec(ctx, insertSaga,
		saga.OrderID, string(saga.PaymentStatus), string(saga.InventoryStatus), saga.CompensationStatus, saga.LastEventType,
	); err != nil {
		return nil, mapWriteError(err, "insert saga state")
	}

	const insertOutbox = `
		INSERT INTO order_outbox (id, event_type, payload)
		VALUES ($1, $2, $3)
	`
	if _, err = tx.Exec(ctx, insertOutbox, outbox.ID, outbox.EventType, outbox.Payload); err != nil {
		return nil, mapWriteError(err, "insert outbox event")
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "commit transaction")
	}

	order.CreatedAt = createdAt
	order.UpdatedAt = updatedAt
	order.Items = items
	return order, nil
}

func (r *pgOrderRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Order, error) {
	const orderQuery = `
		SELECT id, user_id, status, total_amount, currency, COALESCE(failure_reason, ''), COALESCE(idempotency_key, ''), created_at, updated_at
		FROM orders
		WHERE id = $1
	`

	var order model.Order
	err := r.pool.QueryRow(ctx, orderQuery, id).Scan(
		&order.ID, &order.UserID, &order.Status, &order.TotalAmount, &order.Currency,
		&order.FailureReason, &order.IdempotencyKey, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("order not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "find order")
	}

	items, err := r.listItems(ctx, []uuid.UUID{order.ID})
	if err != nil {
		return nil, err
	}
	order.Items = items[order.ID]
	if order.Items == nil {
		order.Items = []model.OrderItem{}
	}
	return &order, nil
}

func (r *pgOrderRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int32) ([]*model.Order, int32, error) {
	const countQuery = `SELECT COUNT(*) FROM orders WHERE user_id = $1`
	var total int32
	if err := r.pool.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "count orders")
	}

	const listQuery = `
		SELECT id, user_id, status, total_amount, currency, COALESCE(failure_reason, ''), COALESCE(idempotency_key, ''), created_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, listQuery, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "list orders")
	}
	defer rows.Close()

	var orders []*model.Order
	var orderIDs []uuid.UUID
	for rows.Next() {
		order := &model.Order{}
		if err = rows.Scan(
			&order.ID, &order.UserID, &order.Status, &order.TotalAmount, &order.Currency,
			&order.FailureReason, &order.IdempotencyKey, &order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "scan order")
		}
		orders = append(orders, order)
		orderIDs = append(orderIDs, order.ID)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "iterate orders")
	}
	if len(orders) == 0 {
		return orders, total, nil
	}

	itemsByOrder, err := r.listItems(ctx, orderIDs)
	if err != nil {
		return nil, 0, err
	}
	for _, order := range orders {
		order.Items = itemsByOrder[order.ID]
		if order.Items == nil {
			order.Items = []model.OrderItem{}
		}
	}
	return orders, total, nil
}

func (r *pgOrderRepository) Cancel(ctx context.Context, id uuid.UUID, reason string, outbox *model.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const cancelOrder = `
		UPDATE orders
		SET status = 'CANCELLED', failure_reason = $2, updated_at = NOW()
		WHERE id = $1 AND status IN ('CREATED', 'PAYMENT_PENDING')
	`
	result, err := tx.Exec(ctx, cancelOrder, id, reason)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "cancel order")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewInvalidInput("order cannot be cancelled in current state")
	}

	const updateSaga = `
		UPDATE order_saga_state
		SET payment_status = 'FAILED', last_event_type = $2, updated_at = NOW()
		WHERE order_id = $1
	`
	if _, err = tx.Exec(ctx, updateSaga, id, outbox.EventType); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "update saga state")
	}

	const insertOutbox = `
		INSERT INTO order_outbox (id, event_type, payload)
		VALUES ($1, $2, $3)
	`
	if _, err = tx.Exec(ctx, insertOutbox, outbox.ID, outbox.EventType, outbox.Payload); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "insert outbox event")
	}

	if err = tx.Commit(ctx); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "commit transaction")
	}
	return nil
}

func (r *pgOrderRepository) ListUnpublishedEvents(ctx context.Context, limit int) ([]*model.OutboxEvent, error) {
	const q = `
		SELECT id, event_type, payload, published, created_at, published_at
		FROM order_outbox
		WHERE published = FALSE
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "list outbox events")
	}
	defer rows.Close()

	var events []*model.OutboxEvent
	for rows.Next() {
		event := &model.OutboxEvent{}
		if err = rows.Scan(&event.ID, &event.EventType, &event.Payload, &event.Published, &event.CreatedAt, &event.PublishedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, err, "scan outbox event")
		}
		events = append(events, event)
	}
	if err = rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "iterate outbox events")
	}
	return events, nil
}

func (r *pgOrderRepository) MarkEventPublished(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE order_outbox
		SET published = TRUE, published_at = NOW()
		WHERE id = $1
	`
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "mark outbox event published")
	}
	return nil
}

func (r *pgOrderRepository) listItems(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]model.OrderItem, error) {
	const q = `
		SELECT id, order_id, product_id, seller_id, product_name, unit_price, quantity, subtotal
		FROM order_items
		WHERE order_id = ANY($1)
		ORDER BY order_id, id
	`
	rows, err := r.pool.Query(ctx, q, orderIDs)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "list order items")
	}
	defer rows.Close()

	itemsByOrder := make(map[uuid.UUID][]model.OrderItem, len(orderIDs))
	for rows.Next() {
		var item model.OrderItem
		if err = rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID, &item.SellerID, &item.ProductName, &item.UnitPrice, &item.Quantity, &item.Subtotal,
		); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, err, "scan order item")
		}
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
	}
	if err = rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "iterate order items")
	}
	return itemsByOrder, nil
}

func mapWriteError(err error, msg string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperrors.NewAlreadyExists("idempotency key already exists")
	}
	return apperrors.Wrap(apperrors.CodeInternal, err, msg)
}

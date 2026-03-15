package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/product/internal/model"
)

type pgProductRepository struct {
	pool *pgxpool.Pool
}

// NewProductRepository creates a new PostgreSQL-backed product repository.
func NewProductRepository(pool *pgxpool.Pool) ProductRepository {
	return &pgProductRepository{pool: pool}
}

// productEventPayload is the inner payload for product outbox events.
type productEventPayload struct {
	ProductID   string   `json:"product_id"`
	SellerID    string   `json:"seller_id"`
	CategoryID  string   `json:"category_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	Stock       int32    `json:"stock"`
	Status      string   `json:"status"`
	Images      []string `json:"images"`
}

type eventEnvelope struct {
	EventID      string          `json:"event_id"`
	EventType    string          `json:"event_type"`
	EventVersion string          `json:"event_version"`
	Source       string          `json:"source"`
	Timestamp    string          `json:"timestamp"`
	Payload      json.RawMessage `json:"payload"`
}

func buildOutboxPayload(eventType string, p *model.Product, stock int32) ([]byte, error) {
	inner, err := json.Marshal(productEventPayload{
		ProductID:   p.ID.String(),
		SellerID:    p.SellerID.String(),
		CategoryID:  p.CategoryID.String(),
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       stock,
		Status:      string(p.Status),
		Images:      p.Images,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal product payload: %w", err)
	}
	envelope := eventEnvelope{
		EventID:      uuid.New().String(),
		EventType:    eventType,
		EventVersion: "1.0",
		Source:       "product-service",
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Payload:      inner,
	}
	return json.Marshal(envelope)
}

func (r *pgProductRepository) Create(ctx context.Context, product *model.Product, initialStock int32) (*model.Product, error) {
	imagesJSON, err := json.Marshal(product.Images)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "marshal images")
	}

	outboxPayload, err := buildOutboxPayload("product.created", product, initialStock)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "build outbox payload")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const qProduct = `
		INSERT INTO products (id, seller_id, category_id, name, description, price, status, images)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, qProduct,
		product.ID, product.SellerID, product.CategoryID,
		product.Name, product.Description, product.Price,
		string(product.Status), imagesJSON,
	).Scan(&createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "insert product")
	}

	const qInventory = `
		INSERT INTO inventories (product_id, stock, reserved_stock)
		VALUES ($1, $2, 0)
	`
	if _, err = tx.Exec(ctx, qInventory, product.ID, initialStock); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "insert inventory")
	}

	const qOutbox = `
		INSERT INTO product_outbox (id, event_type, payload)
		VALUES ($1, $2, $3)
	`
	if _, err = tx.Exec(ctx, qOutbox, uuid.New(), "product.created", outboxPayload); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "insert outbox event")
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "commit transaction")
	}

	return &model.Product{
		ID:          product.ID,
		SellerID:    product.SellerID,
		CategoryID:  product.CategoryID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Status:      product.Status,
		Images:      product.Images,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (r *pgProductRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	const q = `
		SELECT id, seller_id, category_id, name, description, price, status, images, created_at, updated_at
		FROM products
		WHERE id = $1 AND status != 'deleted'
	`
	return r.scanProduct(r.pool.QueryRow(ctx, q, id))
}

func (r *pgProductRepository) Update(ctx context.Context, product *model.Product) (*model.Product, error) {
	imagesJSON, err := json.Marshal(product.Images)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "marshal images")
	}

	inv, err := r.GetInventory(ctx, product.ID)
	if err != nil {
		return nil, err
	}

	outboxPayload, err := buildOutboxPayload("product.updated", product, inv.Stock)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "build outbox payload")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const qUpdate = `
		UPDATE products
		SET name = $2, description = $3, price = $4, status = $5, images = $6, updated_at = NOW()
		WHERE id = $1 AND status != 'deleted'
		RETURNING seller_id, category_id, created_at, updated_at
	`
	var sellerID, categoryID uuid.UUID
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, qUpdate,
		product.ID, product.Name, product.Description,
		product.Price, string(product.Status), imagesJSON,
	).Scan(&sellerID, &categoryID, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("product not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "update product")
	}

	const qOutbox = `
		INSERT INTO product_outbox (id, event_type, payload)
		VALUES ($1, $2, $3)
	`
	if _, err = tx.Exec(ctx, qOutbox, uuid.New(), "product.updated", outboxPayload); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "insert outbox event")
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "commit transaction")
	}

	return &model.Product{
		ID:          product.ID,
		SellerID:    sellerID,
		CategoryID:  categoryID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Status:      product.Status,
		Images:      product.Images,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (r *pgProductRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const qDelete = `
		UPDATE products
		SET status = 'deleted', updated_at = NOW()
		WHERE id = $1 AND status != 'deleted'
		RETURNING id, seller_id, category_id, name, description, price, images
	`
	var (
		pid, sellerID, categoryID uuid.UUID
		name, description         string
		price                     int64
		imagesRaw                 []byte
	)
	err = tx.QueryRow(ctx, qDelete, id).Scan(
		&pid, &sellerID, &categoryID, &name, &description, &price, &imagesRaw,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("product not found")
		}
		return apperrors.Wrap(apperrors.CodeInternal, err, "soft delete product")
	}

	var images []string
	if len(imagesRaw) > 0 {
		if err = json.Unmarshal(imagesRaw, &images); err != nil {
			return apperrors.Wrap(apperrors.CodeInternal, err, "unmarshal images")
		}
	}

	deletedProduct := &model.Product{
		ID:          pid,
		SellerID:    sellerID,
		CategoryID:  categoryID,
		Name:        name,
		Description: description,
		Price:       price,
		Status:      model.ProductStatusDeleted,
		Images:      images,
	}

	outboxPayload, err := buildOutboxPayload("product.deleted", deletedProduct, 0)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "build outbox payload")
	}

	const qOutbox = `
		INSERT INTO product_outbox (id, event_type, payload)
		VALUES ($1, $2, $3)
	`
	if _, err = tx.Exec(ctx, qOutbox, uuid.New(), "product.deleted", outboxPayload); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "insert outbox event")
	}

	return tx.Commit(ctx)
}

func (r *pgProductRepository) List(ctx context.Context, filter ListFilter) ([]*model.Product, int32, error) {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	const q = `
		SELECT id, seller_id, category_id, name, description, price, status, images, created_at, updated_at,
		       COUNT(*) OVER() AS total
		FROM products
		WHERE status != 'deleted'
		  AND ($1::uuid IS NULL OR category_id = $1::uuid)
		  AND ($2::uuid IS NULL OR seller_id = $2::uuid)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	var catParam, sellerParam interface{}
	if filter.CategoryID != nil {
		catParam = filter.CategoryID
	}
	if filter.SellerID != nil {
		sellerParam = filter.SellerID
	}

	rows, err := r.pool.Query(ctx, q, catParam, sellerParam, pageSize, offset)
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "list products")
	}
	defer rows.Close()

	var products []*model.Product
	var total int32
	for rows.Next() {
		var (
			p         model.Product
			imagesRaw []byte
			t         int32
		)
		if err = rows.Scan(
			&p.ID, &p.SellerID, &p.CategoryID,
			&p.Name, &p.Description, &p.Price,
			&p.Status, &imagesRaw,
			&p.CreatedAt, &p.UpdatedAt, &t,
		); err != nil {
			return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "scan product row")
		}
		if len(imagesRaw) > 0 {
			if err = json.Unmarshal(imagesRaw, &p.Images); err != nil {
				return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "unmarshal images")
			}
		}
		total = t
		products = append(products, &p)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "iterate product rows")
	}
	return products, total, nil
}

func (r *pgProductRepository) GetInventory(ctx context.Context, productID uuid.UUID) (*model.Inventory, error) {
	const q = `
		SELECT product_id, stock, reserved_stock, updated_at
		FROM inventories
		WHERE product_id = $1
	`
	var inv model.Inventory
	err := r.pool.QueryRow(ctx, q, productID).Scan(
		&inv.ProductID, &inv.Stock, &inv.ReservedStock, &inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("inventory not found for product %s", productID)
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "get inventory")
	}
	return &inv, nil
}

func (r *pgProductRepository) ListInventories(ctx context.Context, productIDs []uuid.UUID) ([]*model.Inventory, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	const q = `
		SELECT product_id, stock, reserved_stock, updated_at
		FROM inventories
		WHERE product_id = ANY($1::uuid[])
	`
	rows, err := r.pool.Query(ctx, q, productIDs)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "list inventories")
	}
	defer rows.Close()

	var inventories []*model.Inventory
	for rows.Next() {
		var inv model.Inventory
		if err = rows.Scan(&inv.ProductID, &inv.Stock, &inv.ReservedStock, &inv.UpdatedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, err, "scan inventory row")
		}
		inventories = append(inventories, &inv)
	}
	return inventories, rows.Err()
}

func (r *pgProductRepository) ReserveInventory(
	ctx context.Context,
	_ uuid.UUID,
	items []model.InventoryItem,
) (bool, string, []model.InventoryItem, error) {
	if len(items) == 0 {
		return true, "", nil, nil
	}

	productIDs := make([]uuid.UUID, len(items))
	for i, item := range items {
		productIDs[i] = item.ProductID
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, "", nil, apperrors.Wrap(apperrors.CodeInternal, err, "begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const qLock = `
		SELECT product_id, stock, reserved_stock
		FROM inventories
		WHERE product_id = ANY($1::uuid[])
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, qLock, productIDs)
	if err != nil {
		return false, "", nil, apperrors.Wrap(apperrors.CodeInternal, err, "lock inventory rows")
	}

	stockMap := make(map[uuid.UUID]int32)
	reservedMap := make(map[uuid.UUID]int32)
	for rows.Next() {
		var pid uuid.UUID
		var stock, reserved int32
		if err = rows.Scan(&pid, &stock, &reserved); err != nil {
			rows.Close()
			return false, "", nil, apperrors.Wrap(apperrors.CodeInternal, err, "scan inventory row")
		}
		stockMap[pid] = stock
		reservedMap[pid] = reserved
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return false, "", nil, apperrors.Wrap(apperrors.CodeInternal, err, "iterate inventory rows")
	}

	// Check availability for all items
	for _, item := range items {
		stock, ok := stockMap[item.ProductID]
		if !ok {
			return false, fmt.Sprintf("product %s not found", item.ProductID), nil, nil
		}
		available := stock - reservedMap[item.ProductID]
		if available < item.Quantity {
			return false, fmt.Sprintf("insufficient stock for product %s: available %d, requested %d",
				item.ProductID, available, item.Quantity), nil, nil
		}
	}

	// Reserve all items
	const qUpdate = `
		UPDATE inventories
		SET reserved_stock = reserved_stock + $2, updated_at = NOW()
		WHERE product_id = $1
	`
	for _, item := range items {
		if _, err = tx.Exec(ctx, qUpdate, item.ProductID, item.Quantity); err != nil {
			return false, "", nil, apperrors.Wrap(apperrors.CodeInternal, err, "update reserved stock")
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return false, "", nil, apperrors.Wrap(apperrors.CodeInternal, err, "commit transaction")
	}

	return true, "", items, nil
}

func (r *pgProductRepository) ReleaseInventory(
	ctx context.Context,
	_ uuid.UUID,
	items []model.InventoryItem,
) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const q = `
		UPDATE inventories
		SET reserved_stock = GREATEST(0, reserved_stock - $2),
		    stock = GREATEST(0, stock - $2),
		    updated_at = NOW()
		WHERE product_id = $1
	`
	for _, item := range items {
		if _, err = tx.Exec(ctx, q, item.ProductID, item.Quantity); err != nil {
			return apperrors.Wrap(apperrors.CodeInternal, err, "release inventory")
		}
	}
	return tx.Commit(ctx)
}

func (r *pgProductRepository) ListUnpublishedEvents(ctx context.Context, limit int) ([]*model.OutboxEvent, error) {
	const q = `
		SELECT id, event_type, payload, published, created_at, published_at
		FROM product_outbox
		WHERE published = FALSE
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "list unpublished events")
	}
	defer rows.Close()

	var events []*model.OutboxEvent
	for rows.Next() {
		var e model.OutboxEvent
		if err = rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Published, &e.CreatedAt, &e.PublishedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, err, "scan outbox event")
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (r *pgProductRepository) MarkEventPublished(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE product_outbox
		SET published = TRUE, published_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "mark event published")
	}
	return nil
}

func (r *pgProductRepository) scanProduct(row pgx.Row) (*model.Product, error) {
	var (
		p         model.Product
		imagesRaw []byte
	)
	err := row.Scan(
		&p.ID, &p.SellerID, &p.CategoryID,
		&p.Name, &p.Description, &p.Price,
		&p.Status, &imagesRaw,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("product not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "scan product")
	}
	if len(imagesRaw) > 0 {
		if err = json.Unmarshal(imagesRaw, &p.Images); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, err, "unmarshal images")
		}
	}
	return &p, nil
}

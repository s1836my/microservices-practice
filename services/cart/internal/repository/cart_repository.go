package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yourorg/micromart/services/cart/internal/model"
)

const (
	cartKeyPrefix = "cart:"
	cartTTL       = 7 * 24 * time.Hour
)

type CartRepository interface {
	Ping(ctx context.Context) error
	GetCart(ctx context.Context, userID string) ([]*model.CartItem, error)
	UpsertItem(ctx context.Context, userID string, item *model.CartItem) error
	RemoveItem(ctx context.Context, userID, productID string) error
	ClearCart(ctx context.Context, userID string) error
}

type cartRepository struct {
	rdb redis.Cmdable
}

func NewCartRepository(rdb redis.Cmdable) CartRepository {
	return &cartRepository{rdb: rdb}
}

func (r *cartRepository) Ping(ctx context.Context) error {
	return r.rdb.Ping(ctx).Err()
}

func (r *cartRepository) GetCart(ctx context.Context, userID string) ([]*model.CartItem, error) {
	values, err := r.rdb.HGetAll(ctx, cartKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall cart: %w", err)
	}

	items := make([]*model.CartItem, 0, len(values))
	for _, raw := range values {
		item, err := unmarshalItem(raw)
		if err != nil {
			return nil, fmt.Errorf("unmarshal cart item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *cartRepository) UpsertItem(ctx context.Context, userID string, item *model.CartItem) error {
	value, err := marshalItem(item)
	if err != nil {
		return fmt.Errorf("marshal cart item: %w", err)
	}

	key := cartKey(userID)
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, key, cartField(item.ProductID), value)
	pipe.Expire(ctx, key, cartTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert cart item: %w", err)
	}
	return nil
}

func (r *cartRepository) RemoveItem(ctx context.Context, userID, productID string) error {
	if err := r.rdb.HDel(ctx, cartKey(userID), cartField(productID)).Err(); err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}
	return nil
}

func (r *cartRepository) ClearCart(ctx context.Context, userID string) error {
	if err := r.rdb.Del(ctx, cartKey(userID)).Err(); err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}
	return nil
}

func cartKey(userID string) string {
	return cartKeyPrefix + userID
}

func cartField(productID string) string {
	return "product:" + productID
}

func marshalItem(item *model.CartItem) (string, error) {
	payload, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func unmarshalItem(raw string) (*model.CartItem, error) {
	var item model.CartItem
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

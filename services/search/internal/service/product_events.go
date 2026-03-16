package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/yourorg/micromart/services/search/internal/model"
)

// ProductIndexer handles product document mutations in the search store.
type ProductIndexer interface {
	UpsertProduct(ctx context.Context, product *model.ProductDocument) error
	DeleteProduct(ctx context.Context, productID string) error
}

// ProductEventProcessor translates product.events messages into index updates.
type ProductEventProcessor struct {
	indexer ProductIndexer
	log     *slog.Logger
}

// NewProductEventProcessor creates a ProductEventProcessor.
func NewProductEventProcessor(indexer ProductIndexer, log *slog.Logger) *ProductEventProcessor {
	return &ProductEventProcessor{indexer: indexer, log: log}
}

type productEventEnvelope struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

// HandleMessage applies one product event to the search index.
func (p *ProductEventProcessor) HandleMessage(ctx context.Context, msg []byte) error {
	var event productEventEnvelope
	if err := json.Unmarshal(msg, &event); err != nil {
		return fmt.Errorf("unmarshal product event: %w", err)
	}

	switch event.EventType {
	case "product.created", "product.updated":
		var doc model.ProductDocument
		if err := json.Unmarshal(event.Payload, &doc); err != nil {
			return fmt.Errorf("unmarshal product payload: %w", err)
		}
		if len(doc.Images) == 0 {
			doc.Images = []string{}
		}
		return p.indexer.UpsertProduct(ctx, &doc)
	case "product.deleted":
		var payload struct {
			ProductID string `json:"product_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("unmarshal delete payload: %w", err)
		}
		return p.indexer.DeleteProduct(ctx, payload.ProductID)
	default:
		if p.log != nil {
			p.log.Warn("ignoring unknown product event", "event_type", event.EventType)
		}
		return nil
	}
}

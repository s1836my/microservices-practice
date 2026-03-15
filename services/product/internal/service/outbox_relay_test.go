package service_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/micromart/services/product/internal/model"
	"github.com/yourorg/micromart/services/product/internal/service"
)

// --- mock publisher ---

type mockPublisher struct {
	published []struct {
		eventType string
		payload   []byte
	}
	failOnPublish bool
}

func (m *mockPublisher) Publish(_ context.Context, eventType string, payload []byte) error {
	if m.failOnPublish {
		return assert.AnError
	}
	m.published = append(m.published, struct {
		eventType string
		payload   []byte
	}{eventType, payload})
	return nil
}

func (m *mockPublisher) Close() error { return nil }

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestOutboxRelay_RunOnce_PublishesAndMarks(t *testing.T) {
	repo := newMockRepo()
	eventID := uuid.New()
	repo.outbox = append(repo.outbox, &model.OutboxEvent{
		ID:        eventID,
		EventType: "product.created",
		Payload:   []byte(`{"event_type":"product.created"}`),
	})

	pub := &mockPublisher{}
	relay := service.NewOutboxRelay(repo, pub, newTestLogger())

	relay.RunOnce(context.Background())

	require.Len(t, pub.published, 1)
	assert.Equal(t, "product.created", pub.published[0].eventType)

	// Verify marked as published in repo
	for _, e := range repo.outbox {
		if e.ID == eventID {
			assert.True(t, e.Published)
		}
	}
}

func TestOutboxRelay_RunOnce_PublishFails_ContinuesWithNext(t *testing.T) {
	repo := newMockRepo()
	repo.outbox = append(repo.outbox,
		&model.OutboxEvent{ID: uuid.New(), EventType: "product.created", Payload: []byte(`{}`)},
		&model.OutboxEvent{ID: uuid.New(), EventType: "product.updated", Payload: []byte(`{}`)},
	)

	pub := &mockPublisher{failOnPublish: true}
	relay := service.NewOutboxRelay(repo, pub, newTestLogger())

	// Should not panic even when publish fails
	relay.RunOnce(context.Background())
	assert.Len(t, pub.published, 0)
}

func TestOutboxRelay_RunOnce_EmptyOutbox(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	relay := service.NewOutboxRelay(repo, pub, newTestLogger())

	relay.RunOnce(context.Background())
	assert.Len(t, pub.published, 0)
}

func TestNoopPublisher(t *testing.T) {
	p := service.NewNoopPublisher()
	require.NotNil(t, p)

	err := p.Publish(context.Background(), "product.created", []byte(`{}`))
	assert.NoError(t, err)

	err = p.Close()
	assert.NoError(t, err)
}

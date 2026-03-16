package service_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/micromart/services/order/internal/model"
	"github.com/yourorg/micromart/services/order/internal/service"
)

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
	}{eventType: eventType, payload: payload})
	return nil
}

func (m *mockPublisher) Close() error { return nil }

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestOutboxRelay_RunOnce_PublishesAndMarks(t *testing.T) {
	repo := newMockOrderRepo()
	eventID := uuid.New()
	repo.outbox = append(repo.outbox, &model.OutboxEvent{
		ID:        eventID,
		EventType: "order.created",
		Payload:   []byte(`{"event_type":"order.created"}`),
	})

	relay := service.NewOutboxRelay(repo, &mockPublisher{}, newTestLogger())
	relay.RunOnce(context.Background())

	require.True(t, repo.outbox[0].Published)
}

func TestOutboxRelay_RunOnce_PublishFails(t *testing.T) {
	repo := newMockOrderRepo()
	repo.outbox = append(repo.outbox, &model.OutboxEvent{
		ID:        uuid.New(),
		EventType: "order.created",
		Payload:   []byte(`{}`),
	})

	relay := service.NewOutboxRelay(repo, &mockPublisher{failOnPublish: true}, newTestLogger())
	relay.RunOnce(context.Background())

	assert.False(t, repo.outbox[0].Published)
}

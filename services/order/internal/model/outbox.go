package model

import "time"

import "github.com/google/uuid"

type OutboxEvent struct {
	ID          uuid.UUID
	EventType   string
	Payload     []byte
	Published   bool
	CreatedAt   time.Time
	PublishedAt *time.Time
}

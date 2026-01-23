package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Receipt struct {
	MessageID uuid.UUID `json:"message_id"`
	UserID    uuid.UUID `json:"user_id"`
	Status    string    `json:"status"` // delivered, read
	UpdatedAt time.Time `json:"updated_at"`
}

type ReceiptRepository interface {
	Upsert(ctx context.Context, receipt Receipt) error
	ListByMessage(ctx context.Context, messageID uuid.UUID) ([]Receipt, error)
}

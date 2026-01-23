package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID             uuid.UUID      `json:"id"`
	ConversationID uuid.UUID      `json:"conversation_id"`
	SenderID       uuid.UUID      `json:"sender_id"`
	MessageType    string         `json:"message_type"`
	Body           map[string]any `json:"body"`
	ClientMsgID    string         `json:"client_msg_id"`
	CreatedAt      time.Time      `json:"created_at"`
}

type MessageRepository interface {
	// InsertOrGet persists a message with idempotency check using ClientMsgID.
	InsertOrGet(ctx context.Context, msg Message) (Message, bool, error)
	// ListHistory retrieves messages for a conversation.
	ListHistory(ctx context.Context, conversationID uuid.UUID, limit int) ([]Message, error)
}

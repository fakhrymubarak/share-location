package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrForbidden    = errors.New("forbidden")
)

// User represents an authenticated person identified by username.
type User struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	CreatedAt   time.Time
}

type ConversationType string

const (
	ConversationTypeDirect ConversationType = "direct"
	ConversationTypeGroup  ConversationType = "group"
)

// Conversation aggregates members and messages.
type Conversation struct {
	ID            uuid.UUID
	Type          ConversationType
	Title         string
	DirectPairKey string
	CreatedAt     time.Time
	Participants  []uuid.UUID
}

// ConversationMember ties a user to a conversation with metadata.
type ConversationMember struct {
	ConversationID    uuid.UUID
	UserID            uuid.UUID
	Role              string
	JoinedAt          time.Time
	LastReadMessageID *uuid.UUID
}

// Message represents a persisted chat message.
type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Body           map[string]any
	MessageType    string
	ClientMsgID    string
	CreatedAt      time.Time
}

// DeliveryReceipt captures delivery/read status per user.
type DeliveryReceipt struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
	Status    string
	UpdatedAt time.Time
}

package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/fakhry/share-location/backend/internal/features/auth/domain"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

// Common errors.
var (
	ErrNotFound     = shareddomain.ErrNotFound
	ErrInvalidInput = shareddomain.ErrInvalidInput
	ErrForbidden    = shareddomain.ErrForbidden
)

// User represents an authenticated person identified by username.
type User = authdomain.User

type ConversationType string

const (
	ConversationTypeDirect ConversationType = "direct"
	ConversationTypeGroup  ConversationType = "group"
)

// Participant summary for lists.
type Participant struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}

// Conversation aggregates members and messages.
type Conversation struct {
	ID            uuid.UUID
	Type          ConversationType
	Title         string
	DirectPairKey string
	CreatedAt     time.Time
	Participants  []Participant
	LastMessage   *Message
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

// LiveLocationSession tracks active publisher/subscriber session metadata.
type LiveLocationSession struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

// LiveLocationPoint represents the latest location for a stream.
type LiveLocationPoint struct {
	StreamID       uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
	Lat            float64
	Lng            float64
	Accuracy       float64
	Speed          float64
	Heading        float64
	UpdatedAt      time.Time
	ExpiresAt      time.Time
}

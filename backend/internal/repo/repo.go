package repo

import (
	"context"

	"github.com/fakhry/share-location/backend/internal/domain"
	authdomain "github.com/fakhry/share-location/backend/internal/features/auth/domain"
	"github.com/google/uuid"
)

// UserRepository manages user records.
type UserRepository = authdomain.UserRepository

// ConversationRepository manages conversations.
type ConversationRepository interface {
	Create(ctx context.Context, conv domain.Conversation) (domain.Conversation, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Conversation, error)
	FindByDirectPairKey(ctx context.Context, pairKey string) (domain.Conversation, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Conversation, error)
}

// MembershipRepository manages conversation members.
type MembershipRepository interface {
	Add(ctx context.Context, members []domain.ConversationMember) error
	IsMember(ctx context.Context, conversationID, userID uuid.UUID) (bool, error)
	ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]domain.ConversationMember, error)
}

// MessageRepository manages chat messages with idempotency.
type MessageRepository interface {
	InsertOrGet(ctx context.Context, msg domain.Message) (saved domain.Message, created bool, err error)
	ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error)
}

// MessagePublisher publishes messages for fan-out.
type MessagePublisher interface {
	PublishMessage(ctx context.Context, msg domain.Message) error
}

// LiveLocationRepository manages live location sessions and latest points.
type LiveLocationRepository interface {
	UpsertSession(ctx context.Context, session domain.LiveLocationSession) (domain.LiveLocationSession, error)
	GetSession(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationSession, error)
	SavePoint(ctx context.Context, point domain.LiveLocationPoint) error
	GetPoint(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationPoint, error)
}

// LiveLocationPublisher publishes live location updates (e.g., to Redis).
type LiveLocationPublisher interface {
	PublishLiveLocation(ctx context.Context, point domain.LiveLocationPoint) error
}

// ReceiptRepository manages delivery/read receipts.
type ReceiptRepository interface {
	Upsert(ctx context.Context, receipt domain.DeliveryReceipt) error
	ListByMessage(ctx context.Context, messageID uuid.UUID) ([]domain.DeliveryReceipt, error)
}

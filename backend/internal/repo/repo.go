package repo

import (
	"context"

	"github.com/fakhry/share-location/backend/internal/domain"
	authdomain "github.com/fakhry/share-location/backend/internal/features/auth/domain"
	convdomain "github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	"github.com/google/uuid"
)

// UserRepository manages user records.
type UserRepository = authdomain.UserRepository

// ConversationRepository manages conversations.
type ConversationRepository = convdomain.ConversationRepository

// MembershipRepository manages conversation members.
type MembershipRepository = convdomain.MembershipRepository

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

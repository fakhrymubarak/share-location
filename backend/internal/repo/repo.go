package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/fakhry/share-location/backend/internal/domain"
)

// UserRepository manages user records.
type UserRepository interface {
	UpsertByUsername(ctx context.Context, username, displayName string) (domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByUsername(ctx context.Context, username string) (domain.User, error)
}

// ConversationRepository manages conversations.
type ConversationRepository interface {
	Create(ctx context.Context, conv domain.Conversation) (domain.Conversation, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Conversation, error)
	FindByDirectPairKey(ctx context.Context, pairKey string) (domain.Conversation, error)
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

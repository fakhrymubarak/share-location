package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	convdomain "github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	"github.com/fakhry/share-location/backend/internal/features/messaging/domain"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

type MessagePublisher interface {
	Publish(ctx context.Context, msg domain.Message) error
}

type MessagingService struct {
	Messages    domain.MessageRepository
	Memberships convdomain.MembershipRepository
	Publisher   MessagePublisher
	Receipts    domain.ReceiptRepository
	now         func() time.Time
}

func NewMessagingService(messages domain.MessageRepository, memberships convdomain.MembershipRepository, publisher MessagePublisher, receipts domain.ReceiptRepository, now func() time.Time) *MessagingService {
	if now == nil {
		now = time.Now
	}
	return &MessagingService{
		Messages:    messages,
		Memberships: memberships,
		Publisher:   publisher,
		Receipts:    receipts,
		now:         now,
	}
}

type SendChatRequest struct {
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Body           map[string]any
	MessageType    string
	ClientMsgID    string
}

// SendChat validates membership, persists with idempotency, and publishes new messages.
// Returns the stored message and a boolean indicating whether it was newly created.
func (s *MessagingService) SendChat(ctx context.Context, req SendChatRequest) (domain.Message, bool, error) {
	if err := validateSendChat(req); err != nil {
		return domain.Message{}, false, err
	}

	isMember, err := s.Memberships.IsMember(ctx, req.ConversationID, req.SenderID)
	if err != nil {
		return domain.Message{}, false, err
	}
	if !isMember {
		return domain.Message{}, false, shareddomain.ErrForbidden
	}

	msg := domain.Message{
		ID:             uuid.New(),
		ConversationID: req.ConversationID,
		SenderID:       req.SenderID,
		Body:           req.Body,
		MessageType:    req.MessageType,
		ClientMsgID:    req.ClientMsgID,
		CreatedAt:      s.now(),
	}

	stored, created, err := s.Messages.InsertOrGet(ctx, msg)
	if err != nil {
		return domain.Message{}, false, err
	}
	if created && s.Publisher != nil {
		if err := s.Publisher.Publish(ctx, stored); err != nil {
			return domain.Message{}, false, err
		}
	}
	return stored, created, nil
}

// ListHistory returns ordered messages for a conversation limited by count.
func (s *MessagingService) ListHistory(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error) {
	if conversationID == uuid.Nil {
		return nil, shareddomain.ErrInvalidInput
	}
	if limit <= 0 {
		limit = 50
	}
	return s.Messages.ListHistory(ctx, conversationID, limit)
}

type ReceiptRequest struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
	Status    string
}

// UpsertReceipt records delivery/read status.
func (s *MessagingService) UpsertReceipt(ctx context.Context, req ReceiptRequest) error {
	if req.MessageID == uuid.Nil || req.UserID == uuid.Nil || strings.TrimSpace(req.Status) == "" {
		return shareddomain.ErrInvalidInput
	}
	receipt := domain.Receipt{
		MessageID: req.MessageID,
		UserID:    req.UserID,
		Status:    req.Status,
		UpdatedAt: s.now(),
	}
	return s.Receipts.Upsert(ctx, receipt)
}

func validateSendChat(req SendChatRequest) error {
	if req.ConversationID == uuid.Nil || req.SenderID == uuid.Nil {
		return shareddomain.ErrInvalidInput
	}
	if strings.TrimSpace(req.ClientMsgID) == "" || !isSupportedMessageType(req.MessageType) {
		return shareddomain.ErrInvalidInput
	}
	if req.Body == nil {
		return shareddomain.ErrInvalidInput
	}
	return nil
}

func isSupportedMessageType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "text", "location", "media":
		return true
	default:
		return false
	}
}

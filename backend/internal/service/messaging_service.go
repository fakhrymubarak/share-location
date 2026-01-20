package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo"
)

type MessagingService struct {
	Messages    repo.MessageRepository
	Memberships repo.MembershipRepository
	Publisher   repo.MessagePublisher
	now         func() time.Time
}

func NewMessagingService(messages repo.MessageRepository, memberships repo.MembershipRepository, publisher repo.MessagePublisher, now func() time.Time) *MessagingService {
	if now == nil {
		now = time.Now
	}
	return &MessagingService{
		Messages:    messages,
		Memberships: memberships,
		Publisher:   publisher,
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
		return domain.Message{}, false, domain.ErrForbidden
	}

	msg := domain.Message{
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
		if err := s.Publisher.PublishMessage(ctx, stored); err != nil {
			return domain.Message{}, false, err
		}
	}
	return stored, created, nil
}

func validateSendChat(req SendChatRequest) error {
	if req.ConversationID == uuid.Nil || req.SenderID == uuid.Nil {
		return domain.ErrInvalidInput
	}
	if strings.TrimSpace(req.ClientMsgID) == "" || strings.TrimSpace(req.MessageType) == "" {
		return domain.ErrInvalidInput
	}
	if req.Body == nil {
		return domain.ErrInvalidInput
	}
	return nil
}

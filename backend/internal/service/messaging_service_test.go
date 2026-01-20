package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo/memory"
)

func TestSendChat_PersistsAndPublishes(t *testing.T) {
	now := time.Date(2026, 1, 4, 9, 0, 0, 0, time.UTC)
	messageRepo := memory.NewMessageRepository(func() time.Time { return now })
	membershipRepo := memory.NewMembershipRepository(func() time.Time { return now })
	publisher := memory.NewMessagePublisher()

	conversationID := uuid.New()
	senderID := uuid.New()
	_ = membershipRepo.Add(context.Background(), []domain.ConversationMember{
		{ConversationID: conversationID, UserID: senderID, Role: "member", JoinedAt: now},
	})

	svc := NewMessagingService(messageRepo, membershipRepo, publisher, func() time.Time { return now })
	req := SendChatRequest{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hi"},
		MessageType:    "text",
		ClientMsgID:    "c1",
	}

	msg, created, err := svc.SendChat(context.Background(), req)
	if err != nil {
		t.Fatalf("SendChat error: %v", err)
	}
	if !created {
		t.Fatalf("expected message to be newly created")
	}
	if msg.CreatedAt != now {
		t.Fatalf("CreatedAt = %v, want %v", msg.CreatedAt, now)
	}
	if msg.MessageType != "text" || msg.Body["text"] != "hi" {
		t.Fatalf("unexpected message %+v", msg)
	}
	if got := publisher.Published(); len(got) != 1 || got[0].ID != msg.ID {
		t.Fatalf("expected one published message")
	}

	msg2, created2, err := svc.SendChat(context.Background(), req)
	if err != nil {
		t.Fatalf("SendChat duplicate error: %v", err)
	}
	if created2 {
		t.Fatalf("expected duplicate send to be idempotent")
	}
	if msg2.ID != msg.ID {
		t.Fatalf("expected same message ID on duplicate send")
	}
	if len(publisher.Published()) != 1 {
		t.Fatalf("expected publisher to fire once")
	}
}

func TestSendChat_AllowsNilPublisher(t *testing.T) {
	now := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	messageRepo := memory.NewMessageRepository(func() time.Time { return now })
	membershipRepo := memory.NewMembershipRepository(func() time.Time { return now })

	conversationID := uuid.New()
	senderID := uuid.New()
	_ = membershipRepo.Add(context.Background(), []domain.ConversationMember{
		{ConversationID: conversationID, UserID: senderID, Role: "member", JoinedAt: now},
	})

	svc := NewMessagingService(messageRepo, membershipRepo, nil, func() time.Time { return now })
	_, created, err := svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hi"},
		MessageType:    "text",
		ClientMsgID:    "c-nil",
	})
	if err != nil {
		t.Fatalf("SendChat error: %v", err)
	}
	if !created {
		t.Fatalf("expected message to be created")
	}
}

func TestSendChat_ValidatesMembershipAndInput(t *testing.T) {
	now := time.Now()
	messageRepo := memory.NewMessageRepository(nil)
	membershipRepo := memory.NewMembershipRepository(nil)
	svc := NewMessagingService(messageRepo, membershipRepo, nil, func() time.Time { return now })

	conversationID := uuid.New()
	senderID := uuid.New()

	_, _, err := svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hi"},
		MessageType:    "text",
		ClientMsgID:    "c1",
	})
	if err != domain.ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-member, got %v", err)
	}

	_, _, err = svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: uuid.Nil,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hi"},
		MessageType:    "text",
		ClientMsgID:    "",
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput for missing fields, got %v", err)
	}

	_, _, err = svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           nil,
		MessageType:    "text",
		ClientMsgID:    "c2",
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput for nil body, got %v", err)
	}

	_, _, err = svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hi"},
		MessageType:    " ",
		ClientMsgID:    "c3",
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput for blank message type, got %v", err)
	}
}

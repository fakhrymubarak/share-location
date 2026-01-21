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
	receipts := memory.NewReceiptRepository(func() time.Time { return now })

	conversationID := uuid.New()
	senderID := uuid.New()
	_ = membershipRepo.Add(context.Background(), []domain.ConversationMember{
		{ConversationID: conversationID, UserID: senderID, Role: "member", JoinedAt: now},
	})

	svc := NewMessagingService(messageRepo, membershipRepo, publisher, receipts, func() time.Time { return now })
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
	receipts := memory.NewReceiptRepository(func() time.Time { return now })

	conversationID := uuid.New()
	senderID := uuid.New()
	_ = membershipRepo.Add(context.Background(), []domain.ConversationMember{
		{ConversationID: conversationID, UserID: senderID, Role: "member", JoinedAt: now},
	})

	svc := NewMessagingService(messageRepo, membershipRepo, nil, receipts, func() time.Time { return now })
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
	receipts := memory.NewReceiptRepository(nil)
	svc := NewMessagingService(messageRepo, membershipRepo, nil, receipts, func() time.Time { return now })

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

	_, _, err = svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hi"},
		MessageType:    "unsupported",
		ClientMsgID:    "c4",
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput for unsupported message type, got %v", err)
	}
}

func TestListHistoryAndReceipts(t *testing.T) {
	now := time.Date(2026, 1, 4, 11, 0, 0, 0, time.UTC)
	msgRepo := memory.NewMessageRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })
	rcRepo := memory.NewReceiptRepository(func() time.Time { return now })
	svc := NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now })

	convID := uuid.New()
	sender := uuid.New()
	_ = memberRepo.Add(context.Background(), []domain.ConversationMember{
		{ConversationID: convID, UserID: sender, Role: "member", JoinedAt: now},
	})

	// Insert two messages.
	_, _, _ = svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: convID,
		SenderID:       sender,
		Body:           map[string]any{"text": "m1"},
		MessageType:    "text",
		ClientMsgID:    "c1",
	})
	_, _, _ = svc.SendChat(context.Background(), SendChatRequest{
		ConversationID: convID,
		SenderID:       sender,
		Body:           map[string]any{"text": "m2"},
		MessageType:    "text",
		ClientMsgID:    "c2",
	})
	msgs, err := svc.ListHistory(context.Background(), convID, 10)
	if err != nil {
		t.Fatalf("ListHistory error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	// Receipts
	msgID := msgs[0].ID
	userID := uuid.New()
	err = svc.UpsertReceipt(context.Background(), ReceiptRequest{
		MessageID: msgID,
		UserID:    userID,
		Status:    "read",
	})
	if err != nil {
		t.Fatalf("UpsertReceipt error: %v", err)
	}
	list, err := rcRepo.ListByMessage(context.Background(), msgID)
	if err != nil {
		t.Fatalf("ListByMessage error: %v", err)
	}
	if len(list) != 1 || list[0].Status != "read" {
		t.Fatalf("unexpected receipts %+v", list)
	}
}

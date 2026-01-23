package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/features/messaging/domain"
	"github.com/google/uuid"
)

func TestMessageRepository_InsertOrGetAndList(t *testing.T) {
	now := time.Date(2026, 1, 4, 12, 0, 0, 0, time.UTC)
	r := NewMessageRepository(func() time.Time { return now })

	convID := uuid.New()
	senderID := uuid.New()
	msg := domain.Message{
		ConversationID: convID,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hello"},
		MessageType:    "text",
		ClientMsgID:    "c1",
	}
	saved, created, err := r.InsertOrGet(context.Background(), msg)
	if err != nil {
		t.Fatalf("InsertOrGet error: %v", err)
	}
	if !created || saved.ID == uuid.Nil {
		t.Fatalf("expected new message with ID")
	}
	if saved.CreatedAt != now {
		t.Fatalf("CreatedAt = %v, want %v", saved.CreatedAt, now)
	}

	dup, created, err := r.InsertOrGet(context.Background(), msg)
	if err != nil {
		t.Fatalf("InsertOrGet dup error: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate to return created=false")
	}
	if dup.ID != saved.ID {
		t.Fatalf("expected same ID for duplicate")
	}

	// Add another message with later timestamp to verify ordering and limit.
	msg2 := msg
	msg2.ClientMsgID = "c2"
	msg2.CreatedAt = now.Add(time.Minute)
	_, _, _ = r.InsertOrGet(context.Background(), msg2)

	list, err := r.ListHistory(context.Background(), convID, 1)
	if err != nil {
		t.Fatalf("ListHistory error: %v", err)
	}
	if len(list) != 1 || list[0].ClientMsgID != "c1" {
		t.Fatalf("expected ordered list with first message, got %+v", list)
	}
}

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/google/uuid"
)

func TestConversationRepository_CreateAndLookup(t *testing.T) {
	now := time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC)
	r := NewMemoryConversationRepository(func() time.Time { return now })

	conv, err := r.Create(context.Background(), domain.Conversation{
		Type:          domain.ConversationTypeDirect,
		DirectPairKey: "a:b",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if conv.ID == uuid.Nil {
		t.Fatalf("expected conversation ID set")
	}
	if conv.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt set")
	}

	got, err := r.GetByID(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got.DirectPairKey != "a:b" {
		t.Fatalf("DirectPairKey = %s, want a:b", got.DirectPairKey)
	}

	byPair, err := r.FindByDirectPairKey(context.Background(), "a:b")
	if err != nil {
		t.Fatalf("FindByDirectPairKey error: %v", err)
	}
	if byPair.ID != conv.ID {
		t.Fatalf("FindByDirectPairKey ID mismatch")
	}
}

func TestConversationRepository_NotFound(t *testing.T) {
	r := NewMemoryConversationRepository(time.Now)
	if _, err := r.GetByID(context.Background(), uuid.New()); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := r.FindByDirectPairKey(context.Background(), "missing"); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

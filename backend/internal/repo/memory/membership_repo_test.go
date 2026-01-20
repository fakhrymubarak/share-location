package memory

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/google/uuid"
)

func TestMembershipRepository_AddAndList(t *testing.T) {
	now := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	r := NewMembershipRepository(func() time.Time { return now })

	convID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	members := []domain.ConversationMember{
		{ConversationID: convID, UserID: userA, Role: "member", JoinedAt: now},
		{ConversationID: convID, UserID: userB, Role: "member", JoinedAt: now},
	}
	if err := r.Add(context.Background(), members); err != nil {
		t.Fatalf("Add error: %v", err)
	}

	isMember, err := r.IsMember(context.Background(), convID, userA)
	if err != nil {
		t.Fatalf("IsMember error: %v", err)
	}
	if !isMember {
		t.Fatalf("expected userA to be member")
	}
	isMember, _ = r.IsMember(context.Background(), convID, uuid.New())
	if isMember {
		t.Fatalf("unexpected membership for unknown user")
	}

	list, err := r.ListByConversation(context.Background(), convID)
	if err != nil {
		t.Fatalf("ListByConversation error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("members len = %d, want 2", len(list))
	}
}

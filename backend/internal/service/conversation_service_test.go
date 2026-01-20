package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo/memory"
)

func TestCreateDirectConversation_NewAndIdempotent(t *testing.T) {
	now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	users := memory.NewUserRepository(func() time.Time { return now })
	convs := memory.NewConversationRepository(func() time.Time { return now })
	members := memory.NewMembershipRepository(func() time.Time { return now })
	svc := NewConversationService(users, convs, members, func() time.Time { return now })

	req := CreateConversationRequest{
		Type:                 domain.ConversationTypeDirect,
		ParticipantUsernames: []string{"alice", "bob"},
	}
	conv, convMembers, err := svc.CreateConversation(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateConversation error: %v", err)
	}
	if conv.Type != domain.ConversationTypeDirect {
		t.Fatalf("conv.Type = %s, want direct", conv.Type)
	}
	if conv.DirectPairKey == "" {
		t.Fatalf("expected direct pair key")
	}
	if conv.CreatedAt != now {
		t.Fatalf("conv.CreatedAt = %v, want %v", conv.CreatedAt, now)
	}
	if len(convMembers) != 2 {
		t.Fatalf("members len = %d, want 2", len(convMembers))
	}
	for _, m := range convMembers {
		if m.ConversationID != conv.ID {
			t.Fatalf("member conversation ID mismatch")
		}
		if m.JoinedAt != now {
			t.Fatalf("member joined at %v, want %v", m.JoinedAt, now)
		}
	}

	again, _, err := svc.CreateConversation(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateConversation repeat error: %v", err)
	}
	if again.ID != conv.ID {
		t.Fatalf("expected same conversation ID on repeat direct creation")
	}
	list, err := members.ListByConversation(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("ListByConversation error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("members stored = %d, want 2", len(list))
	}
}

func TestCreateGroupConversation(t *testing.T) {
	now := time.Date(2026, 1, 2, 11, 0, 0, 0, time.UTC)
	users := memory.NewUserRepository(func() time.Time { return now })
	convs := memory.NewConversationRepository(func() time.Time { return now })
	members := memory.NewMembershipRepository(func() time.Time { return now })
	svc := NewConversationService(users, convs, members, func() time.Time { return now })

	req := CreateConversationRequest{
		Type:                 domain.ConversationTypeGroup,
		Title:                "Team",
		ParticipantUsernames: []string{"alice", "bob", "carol"},
	}
	conv, convMembers, err := svc.CreateConversation(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateConversation error: %v", err)
	}
	if conv.Type != domain.ConversationTypeGroup {
		t.Fatalf("conv.Type = %s, want group", conv.Type)
	}
	if conv.DirectPairKey != "" {
		t.Fatalf("expected empty pair key for group")
	}
	if len(convMembers) != 3 {
		t.Fatalf("members len = %d, want 3", len(convMembers))
	}
}

func TestCreateDirectConversation_RequiresTwoUniqueParticipants(t *testing.T) {
	users := memory.NewUserRepository(time.Now)
	convs := memory.NewConversationRepository(time.Now)
	members := memory.NewMembershipRepository(time.Now)
	svc := NewConversationService(users, convs, members, time.Now)

	req := CreateConversationRequest{
		Type:                 domain.ConversationTypeDirect,
		ParticipantUsernames: []string{"alice", "alice"},
	}
	if _, _, err := svc.CreateConversation(context.Background(), req); err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestListMembersValidation(t *testing.T) {
	svc := NewConversationService(memory.NewUserRepository(nil), memory.NewConversationRepository(nil), memory.NewMembershipRepository(nil), nil)
	if _, err := svc.ListMembers(context.Background(), uuid.Nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

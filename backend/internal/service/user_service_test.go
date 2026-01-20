package service

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo/memory"
)

func TestHandshake_UpsertsUser(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	repo := memory.NewUserRepository(func() time.Time { return now })
	svc := NewUserService(repo)

	ctx := context.Background()
	user, err := svc.Handshake(ctx, "alice")
	if err != nil {
		t.Fatalf("Handshake error: %v", err)
	}
	if user.ID == (domain.User{}.ID) {
		t.Fatalf("expected user ID to be set")
	}
	if user.Username != "alice" || user.DisplayName != "alice" {
		t.Fatalf("unexpected user %+v", user)
	}
	if !user.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %v, want %v", user.CreatedAt, now)
	}

	again, err := svc.Handshake(ctx, "alice")
	if err != nil {
		t.Fatalf("Handshake 2 error: %v", err)
	}
	if again.ID != user.ID {
		t.Fatalf("expected same user ID on repeat handshake")
	}
}

func TestHandshake_RejectsBlankUsername(t *testing.T) {
	repo := memory.NewUserRepository(time.Now)
	svc := NewUserService(repo)

	if _, err := svc.Handshake(context.Background(), "   "); err == nil {
		t.Fatalf("expected error for blank username")
	} else if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

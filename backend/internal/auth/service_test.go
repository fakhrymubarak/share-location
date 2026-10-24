package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

func TestHandshake_UpsertsUser(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryUserRepository(func() time.Time { return now })
	svc := NewUserService(repo)

	ctx := context.Background()
	user, err := svc.Handshake(ctx, "alice")
	if err != nil {
		t.Fatalf("Handshake error: %v", err)
	}
	if user.ID == (User{}.ID) {
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
	repo := NewMemoryUserRepository(time.Now)
	svc := NewUserService(repo)

	if _, err := svc.Handshake(context.Background(), "   "); err == nil {
		t.Fatalf("expected error for blank username")
	} else if err != shareddomain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestRegister_HashingAndStorage(t *testing.T) {
	repo := NewMemoryUserRepository(time.Now)
	svc := NewUserService(repo)
	ctx := context.Background()

	// Register
	user, err := svc.Register(ctx, "bob", "password123")
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if user.Username != "bob" {
		t.Fatalf("expected username bob")
	}
	// Verify hashed (placeholder logic for now: "hashed:"+password)
	if user.PasswordHash != "hashed:password123" {
		t.Fatalf("unexpected hash: %s", user.PasswordHash)
	}

	// Verify persistence
	stored, err := repo.GetByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetByUsername error: %v", err)
	}
	if stored.ID != user.ID || stored.PasswordHash != "hashed:password123" {
		t.Fatalf("mismatch specific persistence")
	}
}

func TestRegister_Validation(t *testing.T) {
	svc := NewUserService(NewMemoryUserRepository(time.Now))
	ctx := context.Background()

	// Short password
	if _, err := svc.Register(ctx, "bob", "short"); !errors.Is(err, shareddomain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for short password, got %v", err)
	}
	// Blank username
	if _, err := svc.Register(ctx, "", "password123"); !errors.Is(err, shareddomain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for blank username, got %v", err)
	}
}

func TestLogin_SuccessAndFailure(t *testing.T) {
	repo := NewMemoryUserRepository(time.Now)
	svc := NewUserService(repo)
	ctx := context.Background()

	// Seed user
	user, _ := svc.Register(ctx, "user1", "password123")

	// Correct login
	got, err := svc.Login(ctx, "user1", "password123")
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("Login returned wrong user")
	}

	// Wrong password
	if _, err := svc.Login(ctx, "user1", "wrongpass"); !errors.Is(err, shareddomain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for wrong pass, got %v", err)
	}

	// Non-existent user (should map NotFound to Forbidden)
	if _, err := svc.Login(ctx, "ghost", "password123"); !errors.Is(err, shareddomain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for missing user, got %v", err)
	}
	// Verify that internally it was NotFound but we return Forbidden
	_, internalErr := repo.GetByUsername(ctx, "ghost")
	if !errors.Is(internalErr, shareddomain.ErrNotFound) {
		t.Fatalf("repo should return NotFound, got %v", internalErr)
	}
}

package memory

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
)

func TestUserRepository_UpsertAndGet(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	r := NewUserRepository(func() time.Time { return now })

	u1, err := r.UpsertByUsername(context.Background(), "Alice", "Alice")
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if u1.CreatedAt != now {
		t.Fatalf("CreatedAt = %v, want %v", u1.CreatedAt, now)
	}

	u2, err := r.UpsertByUsername(context.Background(), "alice", "alice")
	if err != nil {
		t.Fatalf("Upsert duplicate error: %v", err)
	}
	if u2.ID != u1.ID {
		t.Fatalf("expected same user for duplicate username")
	}

	gotByID, err := r.GetByID(context.Background(), u1.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if gotByID.Username != "Alice" {
		t.Fatalf("GetByID username = %q, want Alice", gotByID.Username)
	}

	gotByName, err := r.GetByUsername(context.Background(), "ALICE")
	if err != nil {
		t.Fatalf("GetByUsername error: %v", err)
	}
	if gotByName.ID != u1.ID {
		t.Fatalf("GetByUsername ID mismatch")
	}
}

func TestUserRepository_NotFound(t *testing.T) {
	r := NewUserRepository(time.Now)

	if _, err := r.GetByID(context.Background(), domain.User{}.ID); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := r.GetByUsername(context.Background(), "missing"); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

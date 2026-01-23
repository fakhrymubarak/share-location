package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/features/location/domain"
	"github.com/google/uuid"
)

func TestLiveLocationRepository_SessionsAndPoints(t *testing.T) {
	now := time.Date(2026, 1, 6, 8, 0, 0, 0, time.UTC)
	repo := NewMemoryLocationRepository(func() time.Time { return now })

	session, err := repo.UpsertSession(context.Background(), domain.LiveLocationSession{
		UserID:    uuid.New(),
		ExpiresAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("UpsertSession error: %v", err)
	}
	if session.ID == uuid.Nil {
		t.Fatalf("expected session ID set")
	}
	if session.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt set")
	}

	gotSession, err := repo.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if gotSession.ID != session.ID {
		t.Fatalf("session ID mismatch")
	}

	point := domain.LiveLocationPoint{
		StreamID:       session.ID,
		UserID:         session.UserID,
		ConversationID: uuid.New(),
		Lat:            1,
		Lng:            2,
		UpdatedAt:      now,
	}
	if err := repo.SavePoint(context.Background(), point); err != nil {
		t.Fatalf("SavePoint error: %v", err)
	}
	gotPoint, err := repo.GetPoint(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetPoint error: %v", err)
	}
	if gotPoint.Lat != 1 || gotPoint.Lng != 2 {
		t.Fatalf("point mismatch %+v", gotPoint)
	}
}

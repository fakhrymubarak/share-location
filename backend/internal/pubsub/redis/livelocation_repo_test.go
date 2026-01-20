package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/domain"
)

func TestLiveLocationRepo_SessionAndPoint(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := NewLiveLocationRepo(client, 15*time.Minute)

	now := time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC)
	streamID := uuid.New()
	session := domain.LiveLocationSession{
		ID:             streamID,
		UserID:         uuid.New(),
		ConversationID: uuid.New(),
		ExpiresAt:      now.Add(10 * time.Minute),
	}
	_, err := repo.UpsertSession(context.Background(), session)
	if err != nil {
		t.Fatalf("UpsertSession error: %v", err)
	}

	gotSession, err := repo.GetSession(context.Background(), streamID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if gotSession.UserID != session.UserID {
		t.Fatalf("session user mismatch")
	}

	point := domain.LiveLocationPoint{
		StreamID:       streamID,
		UserID:         session.UserID,
		ConversationID: session.ConversationID,
		Lat:            1,
		Lng:            2,
		Accuracy:       3,
		Speed:          4,
		Heading:        5,
		UpdatedAt:      now,
		ExpiresAt:      now.Add(5 * time.Minute),
	}
	if err := repo.SavePoint(context.Background(), point); err != nil {
		t.Fatalf("SavePoint error: %v", err)
	}

	gotPoint, err := repo.GetPoint(context.Background(), streamID)
	if err != nil {
		t.Fatalf("GetPoint error: %v", err)
	}
	if gotPoint.Lat != 1 || gotPoint.Lng != 2 {
		t.Fatalf("point mismatch %+v", gotPoint)
	}
}

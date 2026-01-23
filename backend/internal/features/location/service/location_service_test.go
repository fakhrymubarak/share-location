package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	convdomain "github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	convrepo "github.com/fakhry/share-location/backend/internal/features/conversations/repository"
	"github.com/fakhry/share-location/backend/internal/features/location/domain"
	"github.com/fakhry/share-location/backend/internal/features/location/repository"
)

func TestStartSession_ClampsTTLAndRequiresMembership(t *testing.T) {
	now := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)
	sessionRepo := repository.NewMemoryLocationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	publisher := repository.NewMemoryLocationPublisher()
	maxTTL := 10 * time.Minute

	conversationID := uuid.New()
	userID := uuid.New()
	_ = memberRepo.Add(context.Background(), []convdomain.ConversationMember{
		{ConversationID: conversationID, UserID: userID, Role: "member", JoinedAt: now},
	})

	svc := NewLiveLocationService(sessionRepo, memberRepo, publisher, maxTTL, func() time.Time { return now })
	session, err := svc.StartSession(context.Background(), StartSessionRequest{
		UserID:         userID,
		ConversationID: conversationID,
		RequestedTTL:   30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}
	expectedExpiry := now.Add(maxTTL)
	if !session.ExpiresAt.Equal(expectedExpiry) {
		t.Fatalf("ExpiresAt = %v, want %v", session.ExpiresAt, expectedExpiry)
	}
	if session.CreatedAt != now {
		t.Fatalf("CreatedAt = %v, want %v", session.CreatedAt, now)
	}

	// Non-member should be rejected.
	_, err = svc.StartSession(context.Background(), StartSessionRequest{
		UserID:         uuid.New(),
		ConversationID: conversationID,
		RequestedTTL:   5 * time.Minute,
	})
	if err != domain.ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-member, got %v", err)
	}

	_, err = svc.StartSession(context.Background(), StartSessionRequest{
		UserID:         uuid.Nil,
		ConversationID: conversationID,
		RequestedTTL:   5 * time.Minute,
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput for missing user, got %v", err)
	}
}

func TestPublishLocation_ValidatesSessionAndPublishes(t *testing.T) {
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	sessionRepo := repository.NewMemoryLocationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	publisher := repository.NewMemoryLocationPublisher()
	svc := NewLiveLocationService(sessionRepo, memberRepo, publisher, 15*time.Minute, func() time.Time { return now })

	conversationID := uuid.New()
	userID := uuid.New()
	_ = memberRepo.Add(context.Background(), []convdomain.ConversationMember{
		{ConversationID: conversationID, UserID: userID, Role: "member", JoinedAt: now},
	})

	session, err := svc.StartSession(context.Background(), StartSessionRequest{
		UserID:         userID,
		ConversationID: conversationID,
		RequestedTTL:   10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	point, err := svc.PublishLocation(context.Background(), PublishLocationRequest{
		StreamID:       session.ID,
		UserID:         userID,
		ConversationID: conversationID,
		Lat:            1.23,
		Lng:            4.56,
		ExpiresAt:      now.Add(20 * time.Minute), // should clamp to session expiry
	})
	if err != nil {
		t.Fatalf("PublishLocation error: %v", err)
	}
	if point.ExpiresAt.After(session.ExpiresAt) {
		t.Fatalf("point expiry %v should not exceed session expiry %v", point.ExpiresAt, session.ExpiresAt)
	}
	if point.Lat != 1.23 || point.Lng != 4.56 {
		t.Fatalf("unexpected point %+v", point)
	}
	stored, err := sessionRepo.GetPoint(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetPoint error: %v", err)
	}
	if stored.Lat != 1.23 {
		t.Fatalf("stored point mismatch")
	}
	if len(publisher.Published()) != 1 {
		t.Fatalf("expected one published update")
	}

	// Expired session should be rejected.
	expiredSession := domain.LiveLocationSession{
		ID:             uuid.New(),
		UserID:         userID,
		ConversationID: conversationID,
		ExpiresAt:      now.Add(-time.Minute),
	}
	_, _ = sessionRepo.UpsertSession(context.Background(), expiredSession)
	_, err = svc.PublishLocation(context.Background(), PublishLocationRequest{
		StreamID:       expiredSession.ID,
		UserID:         userID,
		ConversationID: conversationID,
		Lat:            0,
		Lng:            0,
	})
	if err != domain.ErrForbidden {
		t.Fatalf("expected ErrForbidden for expired session, got %v", err)
	}

	_, err = svc.PublishLocation(context.Background(), PublishLocationRequest{
		StreamID:       session.ID,
		UserID:         userID,
		ConversationID: uuid.New(), // mismatched conversation
		Lat:            0,
		Lng:            0,
	})
	if err != domain.ErrForbidden {
		t.Fatalf("expected ErrForbidden for mismatched conversation, got %v", err)
	}

	_, err = svc.PublishLocation(context.Background(), PublishLocationRequest{
		StreamID:       uuid.Nil,
		UserID:         userID,
		ConversationID: conversationID,
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput for missing IDs, got %v", err)
	}
}

func TestLiveLocation_DefaultsWithoutConversation(t *testing.T) {
	baseNow := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	sessionRepo := repository.NewMemoryLocationRepository(func() time.Time { return baseNow })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return baseNow })
	publisher := repository.NewMemoryLocationPublisher()

	// Covers default MaxTTL path with custom clock.
	svc := NewLiveLocationService(sessionRepo, memberRepo, publisher, 0, func() time.Time { return baseNow })
	userID := uuid.New()
	session, err := svc.StartSession(context.Background(), StartSessionRequest{
		UserID:         userID,
		ConversationID: uuid.Nil,
		RequestedTTL:   0,
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}
	expected := baseNow.Add(15 * time.Minute)
	if !session.ExpiresAt.Equal(expected) {
		t.Fatalf("expected default TTL expiry %v, got %v", expected, session.ExpiresAt)
	}

	// Session without conversation should still allow publish.
	point, err := svc.PublishLocation(context.Background(), PublishLocationRequest{
		StreamID:       session.ID,
		UserID:         userID,
		ConversationID: uuid.Nil,
		Lat:            7,
		Lng:            8,
	})
	if err != nil {
		t.Fatalf("PublishLocation error: %v", err)
	}
	if point.ConversationID != uuid.Nil {
		t.Fatalf("expected empty conversation in point")
	}

	// Covers default now fallback when clock is nil.
	sessionRepo2 := repository.NewMemoryLocationRepository(nil)
	svc2 := NewLiveLocationService(sessionRepo2, memberRepo, publisher, 0, nil)
	start := time.Now()
	session2, err := svc2.StartSession(context.Background(), StartSessionRequest{
		UserID:         uuid.New(),
		ConversationID: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}
	minExpected := start.Add(15 * time.Minute)
	maxExpected := time.Now().Add(15*time.Minute + time.Second)
	if session2.ExpiresAt.Before(minExpected) || session2.ExpiresAt.After(maxExpected) {
		t.Fatalf("session2 expiry %v outside expected bounds", session2.ExpiresAt)
	}
}

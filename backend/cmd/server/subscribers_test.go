package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/auth"
	"github.com/fakhry/share-location/backend/internal/domain"
	convrepo "github.com/fakhry/share-location/backend/internal/features/conversations/repository"
	convservice "github.com/fakhry/share-location/backend/internal/features/conversations/service"
	"github.com/fakhry/share-location/backend/internal/shared/logging"
)

type stubHub struct {
	payloads chan []byte
}

func (s *stubHub) BroadcastToConversation(ctx context.Context, _ *convservice.ConversationService, _ uuid.UUID, _ string, payload []byte) {
	select {
	case s.payloads <- payload:
	case <-ctx.Done():
	}
}

func TestStartRedisFanoutBroadcastsChat(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis error: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hub := &stubHub{payloads: make(chan []byte, 1)}
	convSvc := convservice.NewConversationService(auth.NewMemoryUserRepository(nil), convrepo.NewMemoryConversationRepository(nil), convrepo.NewMemoryMembershipRepository(nil), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startRedisFanout(ctx, client, hub, convSvc, logging.Nop())
	time.Sleep(50 * time.Millisecond)

	msg := domain.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		SenderID:       uuid.New(),
		MessageType:    "text",
		ClientMsgID:    "c1",
		Body:           map[string]any{"text": "hi"},
		CreatedAt:      time.Now(),
	}
	raw, _ := json.Marshal(msg)
	mr.Publish("conv:"+msg.ConversationID.String()+":messages", string(raw))

	select {
	case payload := <-hub.payloads:
		if !json.Valid(payload) {
			t.Fatalf("expected JSON payload, got %s", string(payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected broadcast payload")
	}
}

func TestStartRedisFanoutBroadcastsLive(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis error: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hub := &stubHub{payloads: make(chan []byte, 1)}
	convSvc := convservice.NewConversationService(auth.NewMemoryUserRepository(nil), convrepo.NewMemoryConversationRepository(nil), convrepo.NewMemoryMembershipRepository(nil), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startRedisFanout(ctx, client, hub, convSvc, logging.Nop())
	time.Sleep(50 * time.Millisecond)

	point := domain.LiveLocationPoint{
		StreamID:       uuid.New(),
		ConversationID: uuid.New(),
		UserID:         uuid.New(),
		Lat:            1.23,
		Lng:            3.21,
		UpdatedAt:      time.Now(),
	}
	raw, _ := json.Marshal(point)
	mr.Publish("live:"+point.ConversationID.String(), string(raw))

	select {
	case payload := <-hub.payloads:
		if !json.Valid(payload) {
			t.Fatalf("expected JSON payload, got %s", string(payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected live broadcast payload")
	}
}

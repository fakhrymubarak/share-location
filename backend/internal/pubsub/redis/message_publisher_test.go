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

func TestMessagePublisher_Publish(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	pub := NewMessagePublisher(client)

	msg := domain.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		SenderID:       uuid.New(),
		Body:           map[string]any{"text": "hi"},
		MessageType:    "text",
		ClientMsgID:    "c1",
		CreatedAt:      time.Now(),
	}
	channel := "conv:" + msg.ConversationID.String() + ":messages"
	sub := client.Subscribe(context.Background(), channel)
	if err := sub.Ping(context.Background()); err != nil {
		t.Fatalf("subscribe ping error: %v", err)
	}
	if err := pub.PublishMessage(context.Background(), msg); err != nil {
		t.Fatalf("PublishMessage error: %v", err)
	}
	m, err := sub.ReceiveMessage(context.Background())
	if err != nil {
		t.Fatalf("receive error: %v", err)
	}
	if m.Channel != channel {
		t.Fatalf("unexpected channel %s", m.Channel)
	}
}

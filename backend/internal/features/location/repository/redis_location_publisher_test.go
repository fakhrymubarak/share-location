package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/features/location/domain"
)

func TestRedisLocationPublisher_Publish(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	pub := NewRedisLocationPublisher(client)

	point := domain.LiveLocationPoint{
		StreamID:       uuid.New(),
		UserID:         uuid.New(),
		ConversationID: uuid.New(),
		Lat:            1,
		Lng:            2,
		UpdatedAt:      time.Now(),
	}
	channel := "live:" + point.ConversationID.String()
	sub := client.Subscribe(context.Background(), channel)
	if err := sub.Ping(context.Background()); err != nil {
		t.Fatalf("subscribe ping error: %v", err)
	}
	if err := pub.PublishLiveLocation(context.Background(), point); err != nil {
		t.Fatalf("PublishLiveLocation error: %v", err)
	}
	msg, err := sub.ReceiveMessage(context.Background())
	if err != nil {
		t.Fatalf("receive error: %v", err)
	}
	if msg.Channel != channel {
		t.Fatalf("unexpected channel %s", msg.Channel)
	}
}

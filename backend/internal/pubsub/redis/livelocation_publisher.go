package redis

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/domain"
)

// LiveLocationPublisher publishes live location updates to a Redis channel.
type LiveLocationPublisher struct {
	Client *redis.Client
}

func NewLiveLocationPublisher(client *redis.Client) *LiveLocationPublisher {
	return &LiveLocationPublisher{Client: client}
}

func (p *LiveLocationPublisher) PublishLiveLocation(ctx context.Context, point domain.LiveLocationPoint) error {
	channel := "live:" + point.ConversationID.String()
	payload, err := json.Marshal(point)
	if err != nil {
		return err
	}
	return p.Client.Publish(ctx, channel, payload).Err()
}

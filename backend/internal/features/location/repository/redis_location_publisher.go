package repository

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/features/location/domain"
)

// RedisLocationPublisher publishes live location updates to a Redis channel.
type RedisLocationPublisher struct {
	Client *redis.Client
}

func NewRedisLocationPublisher(client *redis.Client) *RedisLocationPublisher {
	return &RedisLocationPublisher{Client: client}
}

func (p *RedisLocationPublisher) PublishLiveLocation(ctx context.Context, point domain.LiveLocationPoint) error {
	channel := "live:" + point.ConversationID.String()
	payload, err := json.Marshal(point)
	if err != nil {
		return err
	}
	return p.Client.Publish(ctx, channel, payload).Err()
}

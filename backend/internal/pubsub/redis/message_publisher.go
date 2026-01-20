package redis

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/domain"
)

// MessagePublisher publishes chat messages to Redis channel per conversation.
type MessagePublisher struct {
	Client *redis.Client
}

func NewMessagePublisher(client *redis.Client) *MessagePublisher {
	return &MessagePublisher{Client: client}
}

func (p *MessagePublisher) PublishMessage(ctx context.Context, msg domain.Message) error {
	channel := "conv:" + msg.ConversationID.String() + ":messages"
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return p.Client.Publish(ctx, channel, payload).Err()
}

package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/service"
)

type broadcastHub interface {
	BroadcastToConversation(ctx context.Context, convSvc *service.ConversationService, convID uuid.UUID, excludeConnID string, payload []byte)
}

// startRedisFanout subscribes to Redis channels and rebroadcasts to local connections.
func startRedisFanout(ctx context.Context, client *redis.Client, hub broadcastHub, convSvc *service.ConversationService, logger interface {
	Infof(string, ...any)
	Warnf(string, ...any)
}) {
	if client == nil || hub == nil || convSvc == nil {
		return
	}
	logger.Infof("starting redis fanout subscribers")
	go subscribeMessages(ctx, client, hub, convSvc, logger)
	go subscribeLive(ctx, client, hub, convSvc, logger)
}

func subscribeMessages(ctx context.Context, client *redis.Client, hub broadcastHub, convSvc *service.ConversationService, logger interface {
	Warnf(string, ...any)
}) {
	pubsub := client.PSubscribe(ctx, "conv:*:messages")
	defer pubsub.Close()
	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warnf("redis message subscription error: %v", err)
			continue
		}
		var payload domain.Message
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			logger.Warnf("redis message unmarshal error: %v", err)
			continue
		}
		broadcastChat(ctx, hub, convSvc, payload)
	}
}

func subscribeLive(ctx context.Context, client *redis.Client, hub broadcastHub, convSvc *service.ConversationService, logger interface {
	Warnf(string, ...any)
}) {
	pubsub := client.PSubscribe(ctx, "live:*")
	defer pubsub.Close()
	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warnf("redis live subscription error: %v", err)
			continue
		}
		var payload domain.LiveLocationPoint
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			logger.Warnf("redis live unmarshal error: %v", err)
			continue
		}
		if payload.ConversationID == uuid.Nil {
			continue
		}
		broadcastLive(ctx, hub, convSvc, payload)
	}
}

func broadcastChat(ctx context.Context, hub broadcastHub, convSvc *service.ConversationService, payload domain.Message) {
	out := map[string]any{
		"event": "RECEIVE_CHAT",
		"data": map[string]any{
			"message_id":      payload.ID.String(),
			"conversation_id": payload.ConversationID.String(),
			"sender_id":       payload.SenderID.String(),
			"message_type":    payload.MessageType,
			"client_msg_id":   payload.ClientMsgID,
			"body":            payload.Body,
			"created_at":      payload.CreatedAt.Format(time.RFC3339),
		},
	}
	encoded, _ := json.Marshal(out)
	hub.BroadcastToConversation(ctx, convSvc, payload.ConversationID, "", encoded)
}

func broadcastLive(ctx context.Context, hub broadcastHub, convSvc *service.ConversationService, payload domain.LiveLocationPoint) {
	out := map[string]any{
		"event": "SUBSCRIBE_LIVE_LOCATION",
		"data": map[string]any{
			"stream_id":       payload.StreamID.String(),
			"conversation_id": payload.ConversationID.String(),
			"point": map[string]any{
				"lat":      payload.Lat,
				"lng":      payload.Lng,
				"accuracy": payload.Accuracy,
				"speed":    payload.Speed,
				"heading":  payload.Heading,
			},
			"updated_at": payload.UpdatedAt.Format(time.RFC3339),
		},
	}
	encoded, _ := json.Marshal(out)
	hub.BroadcastToConversation(ctx, convSvc, payload.ConversationID, "", encoded)
}

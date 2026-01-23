package httpserver

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	msgservice "github.com/fakhry/share-location/backend/internal/features/messaging/service"
	"github.com/fakhry/share-location/backend/internal/service"
)

func wsHandlerWithDeps(deps Deps) func(*websocket.Conn) {
	hub := deps.Hub
	baseLog := deps.Logger.WithFields(logrus.Fields{"component": "ws"})
	type envelope struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
		ReqID string          `json:"req_id"`
	}
	type connected struct {
		Event string `json:"event"`
		Data  struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
		} `json:"data"`
		ReqID string `json:"req_id,omitempty"`
	}
	type sendChat struct {
		ConversationID string         `json:"conversation_id"`
		Body           map[string]any `json:"body"`
		MessageType    string         `json:"message_type"`
		ClientMsgID    string         `json:"client_msg_id"`
	}
	type receiveChat struct {
		Event string `json:"event"`
		Data  struct {
			ID             string         `json:"message_id"`
			ConversationID string         `json:"conversation_id"`
			SenderID       string         `json:"sender_id"`
			MessageType    string         `json:"message_type"`
			ClientMsgID    string         `json:"client_msg_id"`
			Body           map[string]any `json:"body"`
			CreatedAt      string         `json:"created_at"`
		} `json:"data"`
		ReqID string `json:"req_id,omitempty"`
	}
	type publishLive struct {
		StreamID       string  `json:"stream_id"`
		ConversationID string  `json:"conversation_id"`
		Lat            float64 `json:"lat"`
		Lng            float64 `json:"lng"`
		Accuracy       float64 `json:"accuracy"`
		Speed          float64 `json:"speed"`
		Heading        float64 `json:"heading"`
		ExpiresAt      string  `json:"expires_at"`
	}
	type liveUpdate struct {
		Event string `json:"event"`
		Data  struct {
			StreamID       string         `json:"stream_id"`
			ConversationID string         `json:"conversation_id"`
			Point          map[string]any `json:"point"`
			UpdatedAt      string         `json:"updated_at"`
		} `json:"data"`
		ReqID string `json:"req_id,omitempty"`
	}
	type subscribeLive struct {
		StreamID       string `json:"stream_id"`
		ConversationID string `json:"conversation_id"`
	}
	return func(c *websocket.Conn) {
		username := c.Query("username")
		if username == "" {
			username = c.Headers("X-Username")
		}
		user, err := deps.Users.Handshake(context.Background(), username)
		if err != nil {
			baseLog.WithFields(logrus.Fields{"event": "ws_handshake"}).Warnf("handshake failed for username=%s: %v", username, err)
			_ = c.WriteMessage(websocket.TextMessage, []byte(`{"event":"ERROR","data":{"code":"invalid_username"}}`))
			_ = c.Close()
			return
		}

		userLog := baseLog.WithUser(user.ID)
		connID := hub.add(user.ID, c)
		c.SetPongHandler(func(string) error {
			hub.heartbeat(user.ID, connID)
			return nil
		})
		defer hub.remove(user.ID, connID)
		resp := connected{Event: "CONNECTED"}
		resp.Data.UserID = user.ID.String()
		resp.Data.Username = user.Username
		payload, _ := json.Marshal(resp)
		_ = c.WriteMessage(websocket.TextMessage, payload)
		userLog.WithFields(logrus.Fields{"event": "ws_connected", "conn_id": connID}).Infof("connected username=%s", username)

		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				userLog.WithFields(logrus.Fields{"event": "ws_read"}).Debugf("read error: %v", err)
				return
			}
			var env envelope
			if err := json.Unmarshal(msg, &env); err != nil {
				userLog.WithFields(logrus.Fields{"event": "ws_unmarshal"}).Warnf("unmarshal error: %v", err)
				continue
			}
			switch env.Event {
			case "SEND_CHAT":
				var req sendChat
				if err := json.Unmarshal(env.Data, &req); err != nil {
					sendError(c, env.ReqID, "invalid_payload", "bad send_chat payload")
					userLog.WithFields(logrus.Fields{"event": "send_chat"}).Warnf("invalid payload: %v", err)
					continue
				}
				convID, err := uuid.Parse(req.ConversationID)
				if err != nil {
					sendError(c, env.ReqID, "invalid_conversation", "invalid conversation_id")
					userLog.WithFields(logrus.Fields{"event": "send_chat", "conversation_id": req.ConversationID}).Warnf("invalid conversation id: %v", err)
					continue
				}
				msgRes, _, err := deps.Messaging.SendChat(context.Background(), msgservice.SendChatRequest{
					ConversationID: convID,
					SenderID:       user.ID,
					Body:           req.Body,
					MessageType:    req.MessageType,
					ClientMsgID:    req.ClientMsgID,
				})
				if err != nil {
					sendError(c, env.ReqID, errorCode(err), err.Error())
					userLog.WithFields(logrus.Fields{"event": "send_chat", "conversation_id": convID}).Warnf("send_chat failed: %v", err)
					continue
				}
				out := receiveChat{Event: "RECEIVE_CHAT", ReqID: env.ReqID}
				out.Data.ID = msgRes.ID.String()
				out.Data.ConversationID = msgRes.ConversationID.String()
				out.Data.SenderID = msgRes.SenderID.String()
				out.Data.MessageType = msgRes.MessageType
				out.Data.ClientMsgID = msgRes.ClientMsgID
				out.Data.Body = msgRes.Body
				out.Data.CreatedAt = msgRes.CreatedAt.Format(time.RFC3339)
				respBytes, _ := json.Marshal(out)
				_ = c.WriteMessage(websocket.TextMessage, respBytes)
				if !deps.RedisFanout {
					broadcast := out
					broadcast.ReqID = ""
					broadcastBytes, _ := json.Marshal(broadcast)
					go hub.BroadcastToConversation(context.Background(), deps.Conversations, msgRes.ConversationID, connID, broadcastBytes)
				}
				userLog.WithFields(logrus.Fields{"event": "send_chat", "conversation_id": convID, "message_id": msgRes.ID}).Debugf("send_chat ok")
			case "PUBLISH_LIVE_LOCATION":
				if deps.LiveLocation == nil {
					continue
				}
				var req publishLive
				if err := json.Unmarshal(env.Data, &req); err != nil {
					sendError(c, env.ReqID, "invalid_payload", "bad live payload")
					userLog.WithFields(logrus.Fields{"event": "publish_live"}).Warnf("invalid payload: %v", err)
					continue
				}
				streamID, err := uuid.Parse(req.StreamID)
				if err != nil {
					sendError(c, env.ReqID, "invalid_stream", "invalid stream_id")
					userLog.WithFields(logrus.Fields{"event": "publish_live", "stream_id": req.StreamID}).Warnf("invalid stream id: %v", err)
					continue
				}
				var convID uuid.UUID
				if req.ConversationID != "" {
					convID, err = uuid.Parse(req.ConversationID)
					if err != nil {
						sendError(c, env.ReqID, "invalid_conversation", "invalid conversation_id")
						userLog.WithFields(logrus.Fields{"event": "publish_live", "conversation_id": req.ConversationID}).Warnf("invalid conversation id: %v", err)
						continue
					}
				}
				var expires time.Time
				if req.ExpiresAt != "" {
					expires, _ = time.Parse(time.RFC3339, req.ExpiresAt)
				}
				point, err := deps.LiveLocation.PublishLocation(context.Background(), service.PublishLocationRequest{
					StreamID:       streamID,
					UserID:         user.ID,
					ConversationID: convID,
					Lat:            req.Lat,
					Lng:            req.Lng,
					Accuracy:       req.Accuracy,
					Speed:          req.Speed,
					Heading:        req.Heading,
					ExpiresAt:      expires,
				})
				if err != nil {
					sendError(c, env.ReqID, errorCode(err), err.Error())
					userLog.WithFields(logrus.Fields{"event": "publish_live", "stream_id": streamID}).Warnf("publish failed: %v", err)
					continue
				}
				out := liveUpdate{Event: "SUBSCRIBE_LIVE_LOCATION", ReqID: env.ReqID}
				out.Data.StreamID = point.StreamID.String()
				out.Data.ConversationID = point.ConversationID.String()
				out.Data.Point = map[string]any{
					"lat":      point.Lat,
					"lng":      point.Lng,
					"accuracy": point.Accuracy,
					"speed":    point.Speed,
					"heading":  point.Heading,
				}
				out.Data.UpdatedAt = point.UpdatedAt.Format(time.RFC3339)
				respBytes, _ := json.Marshal(out)
				_ = c.WriteMessage(websocket.TextMessage, respBytes)
				if !deps.RedisFanout {
					broadcast := out
					broadcast.ReqID = ""
					broadcastBytes, _ := json.Marshal(broadcast)
					go hub.BroadcastToConversation(context.Background(), deps.Conversations, point.ConversationID, connID, broadcastBytes)
				}
				userLog.WithFields(logrus.Fields{"event": "publish_live", "stream_id": streamID}).Debugf("publish live ok")
			case "SUBSCRIBE_LIVE_LOCATION":
				if deps.LiveLocation == nil {
					continue
				}
				var req subscribeLive
				if err := json.Unmarshal(env.Data, &req); err != nil {
					sendError(c, env.ReqID, "invalid_payload", "bad subscribe payload")
					userLog.WithFields(logrus.Fields{"event": "subscribe_live"}).Warnf("invalid payload: %v", err)
					continue
				}
				streamID, err := uuid.Parse(req.StreamID)
				if err != nil {
					sendError(c, env.ReqID, "invalid_stream", "invalid stream_id")
					userLog.WithFields(logrus.Fields{"event": "subscribe_live", "stream_id": req.StreamID}).Warnf("invalid stream id: %v", err)
					continue
				}
				point, err := deps.LiveLocation.Sessions.GetPoint(context.Background(), streamID)
				if err != nil {
					sendError(c, env.ReqID, errorCode(err), err.Error())
					userLog.WithFields(logrus.Fields{"event": "subscribe_live", "stream_id": streamID}).Warnf("stream not found: %v", err)
					continue
				}
				out := liveUpdate{Event: "SUBSCRIBE_LIVE_LOCATION", ReqID: env.ReqID}
				out.Data.StreamID = point.StreamID.String()
				out.Data.ConversationID = point.ConversationID.String()
				out.Data.Point = map[string]any{
					"lat":      point.Lat,
					"lng":      point.Lng,
					"accuracy": point.Accuracy,
					"speed":    point.Speed,
					"heading":  point.Heading,
				}
				out.Data.UpdatedAt = point.UpdatedAt.Format(time.RFC3339)
				respBytes, _ := json.Marshal(out)
				_ = c.WriteMessage(websocket.TextMessage, respBytes)
			default:
				// ignore
			}
		}
	}
}

func sendError(c *websocket.Conn, reqID, code, msg string) {
	payload := map[string]any{
		"event":  "ERROR",
		"req_id": reqID,
		"data": map[string]any{
			"code":    code,
			"message": msg,
		},
	}
	b, _ := json.Marshal(payload)
	_ = c.WriteMessage(websocket.TextMessage, b)
}

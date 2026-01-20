package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/logging"
	"github.com/fakhry/share-location/backend/internal/service"
	"github.com/fakhry/share-location/backend/internal/ws/registry"
)

// Deps aggregates services used by the HTTP/WebSocket layer.
type Deps struct {
	Users         *service.UserService
	Conversations *service.ConversationService
	Messaging     *service.MessagingService
	LiveLocation  *service.LiveLocationService
	Connections   *registry.Registry
	Hub           *ConnectionHub
	RedisFanout   bool
	Logger        *logging.Logger
}

func New(deps Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
	})

	if deps.Hub == nil {
		deps.Hub = NewConnectionHub(deps.Connections)
	}
	if deps.Logger == nil {
		deps.Logger = logging.Nop()
	}

	app.Get("/health", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/v1/login", loginHandler(deps.Users))
	app.Post("/v1/conversations", createConversationHandler(deps.Conversations))
	app.Get("/v1/conversations/:id/messages", listMessagesHandler(deps.Messaging))
	app.Post("/v1/messages/:id/receipt", receiptHandler(deps.Messaging))
	app.Post("/v1/live-locations", startLiveLocationHandler(deps.LiveLocation))

	app.Use("/ws", websocket.New(wsHandlerWithDeps(deps)))

	return app
}

func loginHandler(users *service.UserService) fiber.Handler {
	type request struct {
		Username string `json:"username"`
	}
	type response struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
	}
	return func(c *fiber.Ctx) error {
		var req request
		if err := c.BodyParser(&req); err != nil {
			return fiber.ErrBadRequest
		}
		user, err := users.Handshake(c.Context(), req.Username)
		if err != nil {
			return err
		}
		return c.Status(http.StatusOK).JSON(response{
			UserID:   user.ID.String(),
			Username: user.Username,
		})
	}
}

func createConversationHandler(convs *service.ConversationService) fiber.Handler {
	type request struct {
		Type      string   `json:"type"`
		Title     string   `json:"title"`
		Usernames []string `json:"usernames"`
	}
	type response struct {
		ID        string   `json:"id"`
		Type      string   `json:"type"`
		Title     string   `json:"title"`
		Users     []string `json:"user_ids"`
		PairKey   string   `json:"direct_pair_key,omitempty"`
		CreatedAt string   `json:"created_at"`
	}
	return func(c *fiber.Ctx) error {
		var req request
		if err := c.BodyParser(&req); err != nil {
			return fiber.ErrBadRequest
		}
		conv, members, err := convs.CreateConversation(c.Context(), service.CreateConversationRequest{
			Type:                 domain.ConversationType(req.Type),
			Title:                req.Title,
			ParticipantUsernames: req.Usernames,
		})
		if err != nil {
			return err
		}
		userIDs := make([]string, 0, len(members))
		for _, m := range members {
			userIDs = append(userIDs, m.UserID.String())
		}
		return c.Status(http.StatusOK).JSON(response{
			ID:        conv.ID.String(),
			Type:      string(conv.Type),
			Title:     conv.Title,
			Users:     userIDs,
			PairKey:   conv.DirectPairKey,
			CreatedAt: conv.CreatedAt.Format(time.RFC3339),
		})
	}
}

func listMessagesHandler(msgs *service.MessagingService) fiber.Handler {
	type response struct {
		ID             string         `json:"id"`
		ConversationID string         `json:"conversation_id"`
		SenderID       string         `json:"sender_id"`
		MessageType    string         `json:"message_type"`
		ClientMsgID    string         `json:"client_msg_id"`
		Body           map[string]any `json:"body"`
		CreatedAt      string         `json:"created_at"`
		Extra          map[string]any `json:"-"`
	}
	return func(c *fiber.Ctx) error {
		idStr := c.Params("id")
		convID, err := uuid.Parse(idStr)
		if err != nil {
			return fiber.ErrBadRequest
		}
		limit := c.QueryInt("limit", 50)
		msgsList, err := msgs.ListHistory(c.Context(), convID, limit)
		if err != nil {
			return err
		}
		out := make([]response, 0, len(msgsList))
		for _, m := range msgsList {
			out = append(out, response{
				ID:             m.ID.String(),
				ConversationID: m.ConversationID.String(),
				SenderID:       m.SenderID.String(),
				MessageType:    m.MessageType,
				ClientMsgID:    m.ClientMsgID,
				Body:           m.Body,
				CreatedAt:      m.CreatedAt.Format(time.RFC3339),
			})
		}
		return c.Status(http.StatusOK).JSON(out)
	}
}

func receiptHandler(msgs *service.MessagingService) fiber.Handler {
	type request struct {
		UserID string `json:"user_id"`
		Status string `json:"status"`
	}
	return func(c *fiber.Ctx) error {
		msgID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return fiber.ErrBadRequest
		}
		var req request
		if err := c.BodyParser(&req); err != nil {
			return fiber.ErrBadRequest
		}
		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			return fiber.ErrBadRequest
		}
		if err := msgs.UpsertReceipt(c.Context(), service.ReceiptRequest{
			MessageID: msgID,
			UserID:    userID,
			Status:    req.Status,
		}); err != nil {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	}
}

func startLiveLocationHandler(live *service.LiveLocationService) fiber.Handler {
	type request struct {
		UserID         string `json:"user_id"`
		ConversationID string `json:"conversation_id,omitempty"`
		TTLSeconds     int    `json:"ttl_seconds,omitempty"`
	}
	type response struct {
		StreamID       string `json:"stream_id"`
		ExpiresAt      string `json:"expires_at"`
		UserID         string `json:"user_id"`
		ConversationID string `json:"conversation_id,omitempty"`
	}
	return func(c *fiber.Ctx) error {
		var req request
		if err := c.BodyParser(&req); err != nil {
			return fiber.ErrBadRequest
		}
		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			return fiber.ErrBadRequest
		}
		var convID uuid.UUID
		if req.ConversationID != "" {
			convID, err = uuid.Parse(req.ConversationID)
			if err != nil {
				return fiber.ErrBadRequest
			}
		}
		session, err := live.StartSession(c.Context(), service.StartSessionRequest{
			UserID:         userID,
			ConversationID: convID,
			RequestedTTL:   time.Duration(req.TTLSeconds) * time.Second,
		})
		if err != nil {
			return err
		}
		return c.Status(http.StatusOK).JSON(response{
			StreamID:       session.ID.String(),
			ExpiresAt:      session.ExpiresAt.Format(time.RFC3339),
			UserID:         session.UserID.String(),
			ConversationID: session.ConversationID.String(),
		})
	}
}

func wsHandler(users *service.UserService, messaging *service.MessagingService, live *service.LiveLocationService) func(*websocket.Conn) {
	return wsHandlerWithDeps(Deps{
		Users:        users,
		Messaging:    messaging,
		LiveLocation: live,
	})
}

func wsHandlerWithDeps(deps Deps) func(*websocket.Conn) {
	hub := deps.Hub
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
			deps.Logger.Warnf("handshake failed for username=%s: %v", username, err)
			_ = c.WriteMessage(websocket.TextMessage, []byte(`{"event":"ERROR","data":{"code":"invalid_username"}}`))
			_ = c.Close()
			return
		}

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
		deps.Logger.Infof("websocket connected user=%s conn_id=%s", user.ID, connID)

		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				deps.Logger.Debugf("websocket read error user=%s: %v", user.ID, err)
				return
			}
			var env envelope
			if err := json.Unmarshal(msg, &env); err != nil {
				deps.Logger.Warnf("websocket unmarshal error user=%s: %v", user.ID, err)
				continue
			}
			switch env.Event {
			case "SEND_CHAT":
				var req sendChat
				if err := json.Unmarshal(env.Data, &req); err != nil {
					sendError(c, env.ReqID, "invalid_payload", "bad send_chat payload")
					deps.Logger.Warnf("invalid send_chat payload user=%s: %v", user.ID, err)
					continue
				}
				convID, err := uuid.Parse(req.ConversationID)
				if err != nil {
					sendError(c, env.ReqID, "invalid_conversation", "invalid conversation_id")
					deps.Logger.Warnf("invalid conversation id user=%s: %v", user.ID, err)
					continue
				}
				msgRes, _, err := deps.Messaging.SendChat(context.Background(), service.SendChatRequest{
					ConversationID: convID,
					SenderID:       user.ID,
					Body:           req.Body,
					MessageType:    req.MessageType,
					ClientMsgID:    req.ClientMsgID,
				})
				if err != nil {
					sendError(c, env.ReqID, errorCode(err), err.Error())
					deps.Logger.Warnf("send_chat failed user=%s conv=%s: %v", user.ID, convID, err)
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
				deps.Logger.Debugf("send_chat ok user=%s conv=%s msg=%s", user.ID, convID, msgRes.ID)
			case "PUBLISH_LIVE_LOCATION":
				if deps.LiveLocation == nil {
					continue
				}
				var req publishLive
				if err := json.Unmarshal(env.Data, &req); err != nil {
					sendError(c, env.ReqID, "invalid_payload", "bad live payload")
					deps.Logger.Warnf("invalid live payload user=%s: %v", user.ID, err)
					continue
				}
				streamID, err := uuid.Parse(req.StreamID)
				if err != nil {
					sendError(c, env.ReqID, "invalid_stream", "invalid stream_id")
					deps.Logger.Warnf("invalid stream id user=%s: %v", user.ID, err)
					continue
				}
				var convID uuid.UUID
				if req.ConversationID != "" {
					convID, err = uuid.Parse(req.ConversationID)
					if err != nil {
						sendError(c, env.ReqID, "invalid_conversation", "invalid conversation_id")
						deps.Logger.Warnf("invalid live conversation id user=%s: %v", user.ID, err)
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
					deps.Logger.Warnf("publish live failed user=%s stream=%s: %v", user.ID, streamID, err)
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
				deps.Logger.Debugf("publish live ok user=%s stream=%s", user.ID, streamID)
			case "SUBSCRIBE_LIVE_LOCATION":
				if deps.LiveLocation == nil {
					continue
				}
				var req subscribeLive
				if err := json.Unmarshal(env.Data, &req); err != nil {
					sendError(c, env.ReqID, "invalid_payload", "bad subscribe payload")
					deps.Logger.Warnf("invalid subscribe payload user=%s: %v", user.ID, err)
					continue
				}
				streamID, err := uuid.Parse(req.StreamID)
				if err != nil {
					sendError(c, env.ReqID, "invalid_stream", "invalid stream_id")
					deps.Logger.Warnf("invalid subscribe stream id user=%s: %v", user.ID, err)
					continue
				}
				point, err := deps.LiveLocation.Sessions.GetPoint(context.Background(), streamID)
				if err != nil {
					sendError(c, env.ReqID, errorCode(err), err.Error())
					deps.Logger.Warnf("subscribe stream not found user=%s stream=%s: %v", user.ID, streamID, err)
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

func errorHandler(c *fiber.Ctx, err error) error {
	var status = http.StatusInternalServerError
	if fe, ok := err.(*fiber.Error); ok {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
	}
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
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

func errorCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, domain.ErrForbidden):
		return "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	default:
		return "internal"
	}
}

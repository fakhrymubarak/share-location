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
	"github.com/fakhry/share-location/backend/internal/service"
)

// Deps aggregates services used by the HTTP/WebSocket layer.
type Deps struct {
	Users         *service.UserService
	Conversations *service.ConversationService
	Messaging     *service.MessagingService
}

func New(deps Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
	})

	app.Post("/v1/login", loginHandler(deps.Users))
	app.Post("/v1/conversations", createConversationHandler(deps.Conversations))
	app.Get("/v1/conversations/:id/messages", listMessagesHandler(deps.Messaging))
	app.Post("/v1/messages/:id/receipt", receiptHandler(deps.Messaging))

	app.Use("/ws", websocket.New(wsHandler(deps.Users, deps.Messaging)))

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
		ID             string                 `json:"id"`
		ConversationID string                 `json:"conversation_id"`
		SenderID       string                 `json:"sender_id"`
		MessageType    string                 `json:"message_type"`
		ClientMsgID    string                 `json:"client_msg_id"`
		Body           map[string]any         `json:"body"`
		CreatedAt      string                 `json:"created_at"`
		Extra          map[string]any         `json:"-"`
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

func wsHandler(users *service.UserService, messaging *service.MessagingService) func(*websocket.Conn) {
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
		ConversationID string                 `json:"conversation_id"`
		Body           map[string]any         `json:"body"`
		MessageType    string                 `json:"message_type"`
		ClientMsgID    string                 `json:"client_msg_id"`
	}
	type receiveChat struct {
		Event string `json:"event"`
		Data  struct {
			ID             string                 `json:"message_id"`
			ConversationID string                 `json:"conversation_id"`
			SenderID       string                 `json:"sender_id"`
			MessageType    string                 `json:"message_type"`
			ClientMsgID    string                 `json:"client_msg_id"`
			Body           map[string]any         `json:"body"`
			CreatedAt      string                 `json:"created_at"`
		} `json:"data"`
		ReqID string `json:"req_id,omitempty"`
	}
	return func(c *websocket.Conn) {
		username := c.Query("username")
		if username == "" {
			username = c.Headers("X-Username")
		}
		user, err := users.Handshake(context.Background(), username)
		if err != nil {
			_ = c.WriteMessage(websocket.TextMessage, []byte(`{"event":"ERROR","data":{"code":"invalid_username"}}`))
			_ = c.Close()
			return
		}
		resp := connected{Event: "CONNECTED"}
		resp.Data.UserID = user.ID.String()
		resp.Data.Username = user.Username
		payload, _ := json.Marshal(resp)
		_ = c.WriteMessage(websocket.TextMessage, payload)

		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var env envelope
			if err := json.Unmarshal(msg, &env); err != nil {
				continue
			}
			switch env.Event {
			case "SEND_CHAT":
				var req sendChat
				if err := json.Unmarshal(env.Data, &req); err != nil {
					continue
				}
				convID, err := uuid.Parse(req.ConversationID)
				if err != nil {
					continue
				}
				msgRes, _, err := messaging.SendChat(context.Background(), service.SendChatRequest{
					ConversationID: convID,
					SenderID:       user.ID,
					Body:           req.Body,
					MessageType:    req.MessageType,
					ClientMsgID:    req.ClientMsgID,
				})
				if err != nil {
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
			default:
				// ignore
			}
		}
	}
}

func errorHandler(c *fiber.Ctx, err error) error {
	var status = http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
	}
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
}

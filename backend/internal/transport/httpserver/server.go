package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/service"
)

// Deps aggregates services used by the HTTP/WebSocket layer.
type Deps struct {
	Users         *service.UserService
	Conversations *service.ConversationService
}

func New(deps Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
	})

	app.Post("/v1/login", loginHandler(deps.Users))
	app.Post("/v1/conversations", createConversationHandler(deps.Conversations))

	app.Use("/ws", websocket.New(wsHandler(deps.Users)))

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

func wsHandler(users *service.UserService) func(*websocket.Conn) {
	type connected struct {
		Event string `json:"event"`
		Data  struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
		} `json:"data"`
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

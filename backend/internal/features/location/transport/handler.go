package transport

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/service"
)

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

package transport

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	msgservice "github.com/fakhry/share-location/backend/internal/features/messaging/service"
)

func NewListMessagesHandler(msgs *msgservice.MessagingService) fiber.Handler {
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

func NewReceiptHandler(msgs *msgservice.MessagingService) fiber.Handler {
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
		if err := msgs.UpsertReceipt(c.Context(), msgservice.ReceiptRequest{
			MessageID: msgID,
			UserID:    userID,
			Status:    req.Status,
		}); err != nil {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	}
}

package httpserver

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/service"
)

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

func listConversationsHandler(convs *service.ConversationService) fiber.Handler {
	type participant struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	type lastMessage struct {
		SenderID    string `json:"sender_id"`
		Body        any    `json:"body"`
		MessageType string `json:"message_type"`
		CreatedAt   string `json:"created_at"`
	}
	type response struct {
		ID           string        `json:"id"`
		Type         string        `json:"type"`
		Title        string        `json:"title"`
		PairKey      string        `json:"direct_pair_key,omitempty"`
		CreatedAt    string        `json:"created_at"`
		Participants []participant `json:"participants"`
		LastMessage  *lastMessage  `json:"last_message,omitempty"`
	}
	return func(c *fiber.Ctx) error {
		uidStr := c.Query("user_id")
		if uidStr == "" {
			return fiber.NewError(fiber.StatusBadRequest, "missing user_id query param")
		}
		uid, err := uuid.Parse(uidStr)
		if err != nil {
			return fiber.ErrBadRequest
		}

		list, err := convs.ListConversations(c.Context(), uid)
		if err != nil {
			return err
		}

		out := make([]response, 0, len(list))
		for _, cv := range list {
			parts := make([]participant, 0, len(cv.Participants))
			for _, p := range cv.Participants {
				parts = append(parts, participant{
					ID:       p.ID.String(),
					Username: p.Username,
				})
			}
			var lm *lastMessage
			if cv.LastMessage != nil {
				lm = &lastMessage{
					SenderID:    cv.LastMessage.SenderID.String(),
					Body:        cv.LastMessage.Body,
					MessageType: cv.LastMessage.MessageType,
					CreatedAt:   cv.LastMessage.CreatedAt.Format(time.RFC3339),
				}
			}
			out = append(out, response{
				ID:           cv.ID.String(),
				Type:         string(cv.Type),
				Title:        cv.Title,
				PairKey:      cv.DirectPairKey,
				CreatedAt:    cv.CreatedAt.Format(time.RFC3339),
				Participants: parts,
				LastMessage:  lm,
			})
		}
		return c.Status(http.StatusOK).JSON(out)
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

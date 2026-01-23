package transport

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/fakhry/share-location/backend/internal/features/auth/service"
)

func NewLoginHandler(users *service.UserService) fiber.Handler {
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

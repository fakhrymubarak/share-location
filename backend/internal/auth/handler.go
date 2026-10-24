package auth

import (
	"net/http"

	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
	"github.com/gofiber/fiber/v2"
)

func NewAuthHandlers(users *UserService) fiber.Handler {
	// Group handlers can be done in the router, but here we can return a router or just individual handlers.
	// For now let's keep it simple and return a setup function or just expose methods.
	// Actually, the previous pattern returned a single handler. Let's change this to return a struct or specific handlers.
	return nil
}

type AuthHandler struct {
	ServiceName string
	Users       *UserService
}

func NewHandler(users *UserService) *AuthHandler {
	return &AuthHandler{
		ServiceName: "Auth",
		Users:       users,
	}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	type request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var req request
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	user, err := h.Users.Register(c.Context(), req.Username, req.Password)
	if err != nil {
		if err == shareddomain.ErrInvalidInput {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid input"})
		}
		return err // Global error handler should catch this
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"user_id":  user.ID,
		"username": user.Username,
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	type request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var req request
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	user, err := h.Users.Login(c.Context(), req.Username, req.Password)
	if err != nil {
		if err == shareddomain.ErrForbidden || err == shareddomain.ErrNotFound {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}
		return err
	}
	// For now, return user_id as the token (simple MVP as per docs)
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"token":    user.ID.String(), // Using ID as token for now
		"user_id":  user.ID,
		"username": user.Username,
	})
}

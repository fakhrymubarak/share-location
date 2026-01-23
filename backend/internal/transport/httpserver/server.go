package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	authservice "github.com/fakhry/share-location/backend/internal/features/auth/service"
	authtransport "github.com/fakhry/share-location/backend/internal/features/auth/transport"
	convservice "github.com/fakhry/share-location/backend/internal/features/conversations/service"
	convtransport "github.com/fakhry/share-location/backend/internal/features/conversations/transport"
	locservice "github.com/fakhry/share-location/backend/internal/features/location/service"
	loctransport "github.com/fakhry/share-location/backend/internal/features/location/transport"
	msgservice "github.com/fakhry/share-location/backend/internal/features/messaging/service"
	msgtransport "github.com/fakhry/share-location/backend/internal/features/messaging/transport"
	ws "github.com/fakhry/share-location/backend/internal/features/websocket"
	"github.com/fakhry/share-location/backend/internal/features/websocket/registry"
	wstransport "github.com/fakhry/share-location/backend/internal/features/websocket/transport"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
	"github.com/fakhry/share-location/backend/internal/shared/logging"
)

// Deps aggregates services used by the HTTP/WebSocket layer.
type Deps struct {
	Users         *authservice.UserService
	Conversations *convservice.ConversationService
	Messaging     *msgservice.MessagingService
	LiveLocation  *locservice.LiveLocationService
	Connections   *registry.Registry
	Hub           *ws.ConnectionHub
	RedisFanout   bool
	Logger        *logging.Logger
}

func New(deps Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
	})

	if deps.Hub == nil {
		deps.Hub = ws.NewConnectionHub(deps.Connections)
	}
	if deps.Logger == nil {
		deps.Logger = logging.Nop()
	}

	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		// Capture request body (Fiber stores it in memory)
		reqBody := make([]byte, len(c.Body()))
		copy(reqBody, c.Body())

		err := c.Next()
		latency := time.Since(start)

		fields := logrus.Fields{
			"method":  c.Method(),
			"path":    c.Path(),
			"status":  c.Response().StatusCode(),
			"latency": latency.String(),
			"ip":      c.IP(),
		}

		// Attempt to log bodies as string or json if possible
		if len(reqBody) > 0 {
			var bodyData any
			if err := json.Unmarshal(reqBody, &bodyData); err == nil {
				fields["req_body"] = bodyData
			} else {
				fields["req_body"] = string(reqBody)
			}
		}
		if respBody := c.Response().Body(); len(respBody) > 0 {
			// Limit response log size to avoid spamming huge payloads
			if len(respBody) < 2048 {
				var bodyData any
				if err := json.Unmarshal(respBody, &bodyData); err == nil {
					fields["resp_body"] = bodyData
				} else {
					fields["resp_body"] = string(respBody)
				}
			} else {
				fields["resp_body"] = "<too long>"
			}
		}

		if err != nil {
			fields["error"] = err.Error()
		}
		deps.Logger.WithFields(fields).Debugf("http request")
		return err
	})

	app.Get("/health", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/v1/login", authtransport.NewLoginHandler(deps.Users))
	app.Post("/v1/conversations", convtransport.NewCreateConversationHandler(deps.Conversations))
	app.Get("/v1/conversations", convtransport.NewListConversationsHandler(deps.Conversations))
	app.Get("/v1/conversations/:id/messages", msgtransport.NewListMessagesHandler(deps.Messaging))
	app.Post("/v1/messages/:id/receipt", msgtransport.NewReceiptHandler(deps.Messaging))
	app.Post("/v1/live-locations", loctransport.NewStartLiveLocationHandler(deps.LiveLocation))

	app.Use("/ws", websocket.New(wstransport.NewHandler(wstransport.HandlerDeps{
		Users:         deps.Users,
		Conversations: deps.Conversations,
		Messaging:     deps.Messaging,
		LiveLocation:  deps.LiveLocation,
		Hub:           deps.Hub,
		RedisFanout:   deps.RedisFanout,
		Logger:        deps.Logger,
	})))

	return app
}

func errorHandler(c *fiber.Ctx, err error) error {
	var status = http.StatusInternalServerError
	if fe, ok := err.(*fiber.Error); ok {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}
	switch {
	case errors.Is(err, shareddomain.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, shareddomain.ErrForbidden):
		status = http.StatusForbidden
	}
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, shareddomain.ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, shareddomain.ErrForbidden):
		return "forbidden"
	case errors.Is(err, shareddomain.ErrNotFound):
		return "not_found"
	default:
		return "internal"
	}
}

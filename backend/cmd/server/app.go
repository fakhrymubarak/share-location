package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/fakhry/share-location/backend/internal/config"
	"github.com/fakhry/share-location/backend/internal/repo/memory"
	"github.com/fakhry/share-location/backend/internal/service"
	"github.com/fakhry/share-location/backend/internal/transport/httpserver"
)

func main() {
	cfg := mustLoadConfig()
	app, port := buildApp(cfg)

	addr := ":" + strconv.Itoa(port)
	log.Printf("starting server on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func mustLoadConfig() *config.Config {
	env := os.Getenv("APP_ENV")
	cfg, err := config.Load(env)
	if err != nil {
		log.Printf("warning: failed to load config (%v), using in-memory defaults", err)
		cfg = defaultConfig()
	}
	return cfg
}

func defaultConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Port = 8080
	cfg.Messages.MaxPayloadBytes = 16384
	cfg.LiveLocation.MaxTTLSeconds = 900
	return cfg
}

// buildApp wires dependencies using in-memory repositories. Returns the Fiber app and port.
func buildApp(cfg *config.Config) (*fiber.App, int) {
	usersRepo := memory.NewUserRepository(nil)
	convsRepo := memory.NewConversationRepository(nil)
	membersRepo := memory.NewMembershipRepository(nil)
	msgRepo := memory.NewMessageRepository(nil)
	rcRepo := memory.NewReceiptRepository(nil)
	liveRepo := memory.NewLiveLocationRepository(nil)
	msgPublisher := memory.NewMessagePublisher()
	livePublisher := memory.NewLiveLocationPublisher()

	userSvc := service.NewUserService(usersRepo)
	convSvc := service.NewConversationService(usersRepo, convsRepo, membersRepo, nil)
	msgSvc := service.NewMessagingService(msgRepo, membersRepo, msgPublisher, rcRepo, nil)
	liveSvc := service.NewLiveLocationService(liveRepo, membersRepo, livePublisher, seconds(cfg.LiveLocation.MaxTTLSeconds), nil)

	app := httpserver.New(httpserver.Deps{
		Users:         userSvc,
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
	})

	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}
	return app, port
}

func seconds(n int) time.Duration {
	if n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

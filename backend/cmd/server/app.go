package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/config"
	dbpg "github.com/fakhry/share-location/backend/internal/db/pg"
	pubredis "github.com/fakhry/share-location/backend/internal/pubsub/redis"
	"github.com/fakhry/share-location/backend/internal/repo"
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
	_ = dbpg.DB{} // placeholder wiring hook for future Postgres impls
	redisCli := redisClient(cfg.Redis.URL)

	usersRepo := memory.NewUserRepository(nil)
	convsRepo := memory.NewConversationRepository(nil)
	membersRepo := memory.NewMembershipRepository(nil)
	msgRepo := memory.NewMessageRepository(nil)
	rcRepo := memory.NewReceiptRepository(nil)
	var liveRepo repo.LiveLocationRepository = memory.NewLiveLocationRepository(nil)
	var msgPublisher repo.MessagePublisher = memory.NewMessagePublisher()
	var livePublisher repo.LiveLocationPublisher = memory.NewLiveLocationPublisher()

	if redisCli != nil {
		liveRepo = pubredis.NewLiveLocationRepo(redisCli, seconds(cfg.LiveLocation.MaxTTLSeconds))
		livePublisher = pubredis.NewLiveLocationPublisher(redisCli)
		msgPublisher = pubredis.NewMessagePublisher(redisCli)
	}

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

func redisClient(url string) *redis.Client {
	if url == "" {
		return nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil
	}
	return redis.NewClient(opts)
}

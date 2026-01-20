package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/config"
	dbpg "github.com/fakhry/share-location/backend/internal/db/pg"
	pubredis "github.com/fakhry/share-location/backend/internal/pubsub/redis"
	"github.com/fakhry/share-location/backend/internal/repo"
	"github.com/fakhry/share-location/backend/internal/repo/memory"
	pgrepo "github.com/fakhry/share-location/backend/internal/repo/pg"
	"github.com/fakhry/share-location/backend/internal/service"
	"github.com/fakhry/share-location/backend/internal/transport/httpserver"
	"github.com/fakhry/share-location/backend/internal/ws/registry"
)

var pgxPoolFactory = pgxpool.NewWithConfig

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
	cfg.Server.Websocket.HeartbeatIntervalSeconds = 30
	cfg.Server.Websocket.IdleTimeoutSeconds = 60
	return cfg
}

// buildApp wires dependencies using in-memory repositories. Returns the Fiber app and port.
func buildApp(cfg *config.Config) (*fiber.App, int) {
	_ = dbpg.DB{} // placeholder wiring hook for future Postgres impls
	pgPool := postgresPool(cfg)
	redisCli := redisClient(cfg.Redis.URL)

	usersRepo := repo.UserRepository(memory.NewUserRepository(nil))
	convsRepo := repo.ConversationRepository(memory.NewConversationRepository(nil))
	membersRepo := repo.MembershipRepository(memory.NewMembershipRepository(nil))
	msgRepo := repo.MessageRepository(memory.NewMessageRepository(nil))
	rcRepo := repo.ReceiptRepository(memory.NewReceiptRepository(nil))
	livePoints := memory.NewLiveLocationRepository(nil)
	var liveRepo repo.LiveLocationRepository = livePoints
	var msgPublisher repo.MessagePublisher = memory.NewMessagePublisher()
	var livePublisher repo.LiveLocationPublisher = memory.NewLiveLocationPublisher()
	connRegistry := registry.New(seconds(cfg.Server.Websocket.IdleTimeoutSeconds), nil)

	if pgPool != nil {
		usersRepo = pgrepo.NewUserRepository(pgPool)
		convsRepo = pgrepo.NewConversationRepository(pgPool, nil)
		membersRepo = pgrepo.NewMembershipRepository(pgPool, nil)
		msgRepo = pgrepo.NewMessageRepository(pgPool, nil)
		rcRepo = pgrepo.NewReceiptRepository(pgPool)
		liveRepo = pgrepo.NewLiveLocationRepository(pgPool, livePoints, nil)
	}
	if redisCli != nil {
		msgPublisher = pubredis.NewMessagePublisher(redisCli)
		redisLive := pubredis.NewLiveLocationRepo(redisCli, seconds(cfg.LiveLocation.MaxTTLSeconds))
		livePublisher = pubredis.NewLiveLocationPublisher(redisCli)
		if pgLive, ok := liveRepo.(*pgrepo.LiveLocationRepository); ok {
			pgLive.Points = redisLive
		} else {
			liveRepo = redisLive
		}
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
		Connections:   connRegistry,
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

func postgresPool(cfg *config.Config) *pgxpool.Pool {
	if cfg.Database.URL == "" {
		return nil
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		log.Printf("warning: failed to parse database url: %v", err)
		return nil
	}
	if cfg.Database.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 {
		poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetimeMins > 0 {
		poolCfg.MaxConnLifetime = time.Duration(cfg.Database.ConnMaxLifetimeMins) * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxPoolFactory(ctx, poolCfg)
	if err != nil {
		log.Printf("warning: failed to connect to postgres: %v", err)
		return nil
	}
	return pool
}

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
	"github.com/sirupsen/logrus"

	"github.com/fakhry/share-location/backend/internal/auth"
	ws "github.com/fakhry/share-location/backend/internal/features/websocket"
	"github.com/fakhry/share-location/backend/internal/features/websocket/registry"
	pubredis "github.com/fakhry/share-location/backend/internal/pubsub/redis"
	"github.com/fakhry/share-location/backend/internal/shared/config"
	"github.com/fakhry/share-location/backend/internal/shared/logging"
	"github.com/fakhry/share-location/backend/internal/transport/httpserver"

	convdomain "github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	convrepo "github.com/fakhry/share-location/backend/internal/features/conversations/repository"
	convservice "github.com/fakhry/share-location/backend/internal/features/conversations/service"

	locdomain "github.com/fakhry/share-location/backend/internal/features/location/domain"
	locrepo "github.com/fakhry/share-location/backend/internal/features/location/repository"
	locservice "github.com/fakhry/share-location/backend/internal/features/location/service"

	msgdomain "github.com/fakhry/share-location/backend/internal/features/messaging/domain"
	msgrepo "github.com/fakhry/share-location/backend/internal/features/messaging/repository"
	msgservice "github.com/fakhry/share-location/backend/internal/features/messaging/service"
)

var pgxPoolFactory = pgxpool.NewWithConfig

func main() {
	cfg := mustLoadConfig()

	logger := logging.New(cfg.Logging.Level, cfg.Logging.JSON)
	app, port := buildApp(cfg, logger)

	addr := ":" + strconv.Itoa(port)
	logger.WithFields(logrus.Fields{"event": "server_start"}).Infof("starting server on %s", addr)
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

	// Print all used config values.
	println("************************ ALL CONFIG VALUES************************")
	println("[DEBUG] App.Env:", cfg.App.Env)
	println("[DEBUG] App.Name:", cfg.App.Name)
	println("[DEBUG] Server.Port:", cfg.Server.Port)
	println("[DEBUG] Server.MaxRequestBodyBytes:", cfg.Server.MaxRequestBodyBytes)
	println("[DEBUG] Server.Websocket.HeartbeatIntervalSeconds:", cfg.Server.Websocket.HeartbeatIntervalSeconds)
	println("[DEBUG] Server.Websocket.IdleTimeoutSeconds:", cfg.Server.Websocket.IdleTimeoutSeconds)
	println("[DEBUG] Database.URL:", cfg.Database.URL)
	println("[DEBUG] Database.MaxOpenConns:", cfg.Database.MaxOpenConns)
	println("[DEBUG] Database.MaxIdleConns:", cfg.Database.MaxIdleConns)
	println("[DEBUG] Database.ConnMaxLifetimeMins:", cfg.Database.ConnMaxLifetimeMins)
	println("[DEBUG] Redis.URL:", cfg.Redis.URL)
	println("[DEBUG] Redis.ConnectionPoolSize:", cfg.Redis.ConnectionPoolSize)
	println("[DEBUG] Messages.MaxPayloadBytes:", cfg.Messages.MaxPayloadBytes)
	println("[DEBUG] Messages.MaxInFlightPerConnection:", cfg.Messages.MaxInFlightPerConnection)
	println("[DEBUG] LiveLocation.MaxTTLSeconds:", cfg.LiveLocation.MaxTTLSeconds)
	println("[DEBUG] LiveLocation.StrictTTL:", cfg.LiveLocation.StrictTTL)
	println("[DEBUG] LiveLocation.PublishRatePerMinute:", cfg.LiveLocation.PublishRatePerMinute)
	println("[DEBUG] Logging.Level:", cfg.Logging.Level)
	println("[DEBUG] Logging.JSON:", cfg.Logging.JSON)

	return cfg
}

func defaultConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Port = 8080
	cfg.Messages.MaxPayloadBytes = 16384
	cfg.LiveLocation.MaxTTLSeconds = 900
	cfg.Server.Websocket.HeartbeatIntervalSeconds = 30
	cfg.Server.Websocket.IdleTimeoutSeconds = 60
	cfg.Logging.Level = "info"
	return cfg
}

// buildApp wires dependencies using in-memory repositories. Returns the Fiber app and port.
func buildApp(cfg *config.Config, logger *logging.Logger) (*fiber.App, int) {
	pgPool := postgresPool(cfg, logger)
	redisCli := redisClient(cfg.Redis.URL, logger)

	var usersRepo auth.UserRepository = auth.NewMemoryUserRepository(nil)
	var convsRepo convdomain.ConversationRepository = convrepo.NewMemoryConversationRepository(nil)
	var membersRepo convdomain.MembershipRepository = convrepo.NewMemoryMembershipRepository(nil)

	// Messaging feature repositories
	var msgRepo msgdomain.MessageRepository = msgrepo.NewMessageRepository(nil)
	var rcRepo msgdomain.ReceiptRepository = msgrepo.NewReceiptRepository(nil)
	var msgPublisher msgservice.MessagePublisher = msgrepo.NewMessagePublisher()

	// Live location
	livePoints := locrepo.NewMemoryLocationRepository(nil)
	var liveRepo locdomain.LiveLocationRepository = livePoints
	var livePublisher locdomain.LiveLocationPublisher = locrepo.NewMemoryLocationPublisher()

	connRegistry := registry.New(seconds(cfg.Server.Websocket.IdleTimeoutSeconds), nil)
	hub := ws.NewConnectionHub(connRegistry)
	useRedisFanout := false

	if pgPool != nil {
		usersRepo = auth.NewPgUserRepository(pgPool)
		convsRepo = convrepo.NewPgConversationRepository(pgPool, nil)
		membersRepo = convrepo.NewPgMembershipRepository(pgPool, nil)

		msgRepo = msgrepo.NewPgMessageRepository(pgPool, nil)
		rcRepo = msgrepo.NewPgReceiptRepository(pgPool)

		liveRepo = locrepo.NewPgLocationRepository(pgPool, livePoints, nil)
	}
	if redisCli != nil {
		// msgPublisher = pubredis.NewMessagePublisher(redisCli) // Disabled until refactored

		redisLive := pubredis.NewLiveLocationRepo(redisCli, seconds(cfg.LiveLocation.MaxTTLSeconds))
		livePublisher = pubredis.NewLiveLocationPublisher(redisCli)
		if pgLive, ok := liveRepo.(*locrepo.PgLocationRepository); ok {
			pgLive.Points = redisLive // Assumes Points field exists and is public/compatible
		} else {
			liveRepo = redisLive
		}
		useRedisFanout = true
	}

	userSvc := auth.NewUserService(usersRepo)
	convSvc := convservice.NewConversationService(usersRepo, convsRepo, membersRepo, nil)
	msgSvc := msgservice.NewMessagingService(msgRepo, membersRepo, msgPublisher, rcRepo, nil)
	liveSvc := locservice.NewLiveLocationService(liveRepo, membersRepo, livePublisher, seconds(cfg.LiveLocation.MaxTTLSeconds), nil)

	app := httpserver.New(httpserver.Deps{
		Users:         userSvc,
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
		Connections:   connRegistry,
		Hub:           hub,
		RedisFanout:   useRedisFanout,
		Logger:        logger,
	})

	if useRedisFanout && redisCli != nil {
		ctx := context.Background()
		startRedisFanout(ctx, redisCli, hub, convSvc, logger)
	}

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

func redisClient(url string, logger *logging.Logger) *redis.Client {
	if url == "" {
		return nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		logger.WithFields(logrus.Fields{"event": "redis_connect"}).Warnf("failed to parse redis url: %v", err)
		return nil
	}
	client := redis.NewClient(opts)
	logger.WithFields(logrus.Fields{"event": "redis_connect"}).Infof("connected to redis: %s", opts.Addr)
	return client
}

func postgresPool(cfg *config.Config, logger *logging.Logger) *pgxpool.Pool {
	if cfg.Database.URL == "" {
		return nil
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		logger.WithFields(logrus.Fields{"event": "postgres_connect"}).Warnf("failed to parse database url: %v", err)
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
		logger.WithFields(logrus.Fields{"event": "postgres_connect"}).Warnf("failed to connect to postgres: %v", err)
		return nil
	}
	logger.WithFields(logrus.Fields{"event": "postgres_connect"}).Infof("connected to postgres")
	return pool
}

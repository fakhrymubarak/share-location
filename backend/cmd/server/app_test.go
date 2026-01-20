package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fakhry/share-location/backend/internal/logging"
)

func TestDefaultConfigAndBuildApp(t *testing.T) {
	cfg := defaultConfig()
	app, port := buildApp(cfg, logging.Nop())
	if app == nil {
		t.Fatalf("expected app instance")
	}
	if port != 8080 {
		t.Fatalf("expected default port 8080, got %d", port)
	}
}

func TestBuildAppRespectsPort(t *testing.T) {
	cfg := defaultConfig()
	cfg.Server.Port = 9090
	app, port := buildApp(cfg, logging.Nop())
	if app == nil {
		t.Fatalf("expected app instance")
	}
	if port != 9090 {
		t.Fatalf("expected port 9090, got %d", port)
	}
}

func TestSecondsHelper(t *testing.T) {
	if got := seconds(15); got.String() != "15s" {
		t.Fatalf("expected 15s, got %s", got)
	}
	if got := seconds(0); got != 0 {
		t.Fatalf("expected 0 duration, got %s", got)
	}
	if got := seconds(-1); got != 0 {
		t.Fatalf("expected 0 for negative, got %s", got)
	}
}

func TestMustLoadConfigFallback(t *testing.T) {
	t.Setenv("APP_ENV", "nonexistent")
	cfg := mustLoadConfig()
	if cfg.Server.Port == 0 {
		t.Fatalf("expected default port from fallback config")
	}
}

func TestRedisClientParseError(t *testing.T) {
	if client := redisClient("://bad", logging.Nop()); client != nil {
		t.Fatalf("expected nil client on bad URL")
	}
}

func TestRedisClientParseSuccess(t *testing.T) {
	client := redisClient("redis://localhost:6379/0", logging.Nop())
	if client == nil {
		t.Fatalf("expected client")
	}
	_ = client.Close()
}

func TestPostgresPoolParseError(t *testing.T) {
	cfg := defaultConfig()
	cfg.Database.URL = "://bad"
	if pool := postgresPool(cfg, logging.Nop()); pool != nil {
		t.Fatalf("expected nil pool on parse error")
	}
}

func TestPostgresPoolWithStubbedFactory(t *testing.T) {
	cfg := defaultConfig()
	cfg.Database.URL = "postgres://user:pass@localhost:5432/chatapp"
	orig := pgxPoolFactory
	defer func() { pgxPoolFactory = orig }()

	var called bool
	pgxPoolFactory = func(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
		called = true
		return &pgxpool.Pool{}, nil
	}

	if pool := postgresPool(cfg, logging.Nop()); pool == nil {
		t.Fatalf("expected pool from stub factory")
	}
	if !called {
		t.Fatalf("expected stub factory to be used")
	}
}

func TestBuildAppWithPostgresAndRedis(t *testing.T) {
	cfg := defaultConfig()
	cfg.Database.URL = "postgres://user:pass@localhost:5432/chatapp"
	cfg.Redis.URL = "redis://localhost:6379/0"

	orig := pgxPoolFactory
	defer func() { pgxPoolFactory = orig }()
	pgxPoolFactory = func(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
		return &pgxpool.Pool{}, nil
	}

	app, port := buildApp(cfg, logging.Nop())
	if app == nil {
		t.Fatalf("expected app instance")
	}
	if port == 0 {
		t.Fatalf("expected non-zero port")
	}
}

func TestPostgresPoolAppliesLimits(t *testing.T) {
	cfg := defaultConfig()
	cfg.Database.URL = "postgres://user:pass@localhost:5432/chatapp"
	cfg.Database.MaxOpenConns = 20
	cfg.Database.MaxIdleConns = 5
	cfg.Database.ConnMaxLifetimeMins = 10

	orig := pgxPoolFactory
	defer func() { pgxPoolFactory = orig }()

	var seenCfg *pgxpool.Config
	pgxPoolFactory = func(ctx context.Context, poolCfg *pgxpool.Config) (*pgxpool.Pool, error) {
		seenCfg = poolCfg
		return &pgxpool.Pool{}, nil
	}

	if pool := postgresPool(cfg, logging.Nop()); pool == nil {
		t.Fatalf("expected pool")
	}
	if seenCfg == nil {
		t.Fatalf("expected config to be passed to factory")
	}
	if seenCfg.MaxConns != 20 {
		t.Fatalf("max conns not applied")
	}
	if seenCfg.MinConns != 5 {
		t.Fatalf("min conns not applied")
	}
	if seenCfg.MaxConnLifetime != 10*time.Minute {
		t.Fatalf("max lifetime not applied")
	}
}

package main

import (
	"testing"
)

func TestDefaultConfigAndBuildApp(t *testing.T) {
	cfg := defaultConfig()
	app, port := buildApp(cfg)
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
	app, port := buildApp(cfg)
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
	if client := redisClient("://bad"); client != nil {
		t.Fatalf("expected nil client on bad URL")
	}
}

func TestRedisClientParseSuccess(t *testing.T) {
	client := redisClient("redis://localhost:6379/0")
	if client == nil {
		t.Fatalf("expected client")
	}
	_ = client.Close()
}

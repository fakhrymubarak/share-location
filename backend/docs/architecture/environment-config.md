# Environment Configuration

- Status: Draft
- Owner: Backend Team
- Last updated: 20/01/2026

## Overview
Environment-scoped YAML configs for the backend. A Go loader merges a common default with environment overrides (`local` uses defaults, `dev-qa` and `production` override selectively). Settings map directly to documented architecture knobs (DB/Redis URLs, TTLs, heartbeats, payload limits).

## Directory layout (expected)
```
config/
  common/
    default-config.yaml   # base defaults (used by local)
  dev-qa/
    config.yaml           # overrides for dev/QA
  production/
    config.yaml           # overrides for production
```
These files are not checked in; create them from the samples below per environment.

## Key settings (shared schema)
- `app`: `env`, `name`
- `server`: `port`, `max_request_body_bytes`, `websocket.heartbeat_interval_seconds`, `websocket.idle_timeout_seconds`
- `database`: `url`, `max_open_conns`, `max_idle_conns`, `conn_max_lifetime_minutes`
- `redis`: `url`, `connection_pool_size`
- `messages`: `max_payload_bytes`, `max_inflight_per_connection`
- `live_location`: `max_ttl_seconds`, `strict_ttl`, `publish_rate_per_minute`
- `logging`: `level`, `json`

Defaults should live in `config/common/default-config.yaml`; env files override only what differs.

## Sample configs (create locally)
- Common defaults (`config/common/default-config.yaml`)
```yaml
app:
  env: local
  name: share-location-backend
server:
  port: 8080
  max_request_body_bytes: 1048576
  websocket:
    heartbeat_interval_seconds: 30
    idle_timeout_seconds: 60
database:
  url: postgres://chatapp:chatapp@localhost:5432/chatapp?sslmode=disable
  max_open_conns: 20
  max_idle_conns: 10
  conn_max_lifetime_minutes: 30
redis:
  url: redis://localhost:6379/0
  connection_pool_size: 20
messages:
  max_payload_bytes: 16384
  max_inflight_per_connection: 32
live_location:
  max_ttl_seconds: 900
  strict_ttl: false
  publish_rate_per_minute: 120
logging:
  level: info
  json: false
```

- Dev/QA overrides (`config/dev-qa/config.yaml`)
```yaml
app:
  env: dev-qa
server:
  port: 8080
database:
  url: postgres://chatapp:chatapp@dev-db:5432/chatapp?sslmode=disable
  max_open_conns: 40
  max_idle_conns: 20
redis:
  url: redis://dev-redis:6379/0
  connection_pool_size: 40
messages:
  max_payload_bytes: 16384
  max_inflight_per_connection: 32
live_location:
  max_ttl_seconds: 900
  strict_ttl: true
  publish_rate_per_minute: 120
logging:
  level: info
  json: true
```

- Production overrides (`config/production/config.yaml`)
```yaml
app:
  env: production
server:
  port: 8080
database:
  url: postgres://chatapp:change-me@prod-db:5432/chatapp?sslmode=require
  max_open_conns: 80
  max_idle_conns: 30
  conn_max_lifetime_minutes: 30
redis:
  url: rediss://prod-redis:6379/0
  connection_pool_size: 80
messages:
  max_payload_bytes: 32768
  max_inflight_per_connection: 64
live_location:
  max_ttl_seconds: 900
  strict_ttl: true
  publish_rate_per_minute: 120
logging:
  level: info
  json: true
```

## Go loading (example)
```go
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Env  string `yaml:"env"`
		Name string `yaml:"name"`
	} `yaml:"app"`
	Server struct {
		Port                 int `yaml:"port"`
		MaxRequestBodyBytes  int `yaml:"max_request_body_bytes"`
		Websocket struct {
			HeartbeatIntervalSeconds int `yaml:"heartbeat_interval_seconds"`
			IdleTimeoutSeconds       int `yaml:"idle_timeout_seconds"`
		} `yaml:"websocket"`
	} `yaml:"server"`
	Database struct {
		URL                   string `yaml:"url"`
		MaxOpenConns          int    `yaml:"max_open_conns"`
		MaxIdleConns          int    `yaml:"max_idle_conns"`
		ConnMaxLifetimeMins   int    `yaml:"conn_max_lifetime_minutes"`
	} `yaml:"database"`
	Redis struct {
		URL                string `yaml:"url"`
		ConnectionPoolSize int    `yaml:"connection_pool_size"`
	} `yaml:"redis"`
	Messages struct {
		MaxPayloadBytes           int `yaml:"max_payload_bytes"`
		MaxInFlightPerConnection  int `yaml:"max_inflight_per_connection"`
	} `yaml:"messages"`
	LiveLocation struct {
		MaxTTLSeconds         int  `yaml:"max_ttl_seconds"`
		StrictTTL             bool `yaml:"strict_ttl"`
		PublishRatePerMinute  int  `yaml:"publish_rate_per_minute"`
	} `yaml:"live_location"`
	Logging struct {
		Level string `yaml:"level"`
		JSON  bool   `yaml:"json"`
	} `yaml:"logging"`
}

func Load(env string) (*Config, error) {
	base := filepath.Join("config", "common", "default-config.yaml")
	envFile := filepath.Join("config", env, "config.yaml")

	cfg := &Config{}
	if err := loadYAML(base, cfg); err != nil {
		return nil, err
	}
	// Overlay env-specific if present.
	if _, err := os.Stat(envFile); err == nil {
		if err := loadYAML(envFile, cfg); err != nil {
			return nil, err
		}
	}
	// Allow env var override of the DB/Redis URLs if needed.
	if v := os.Getenv("DB_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("REDIS_URL"); v != "" {
		cfg.Redis.URL = v
	}
	return cfg, nil
}

func loadYAML(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}
```

## Usage
- Set `APP_ENV` to `local` (default), `dev-qa`, or `production`.
- Load config: `cfg, err := config.Load(os.Getenv("APP_ENV"))`.
- Override secrets/URLs with environment variables in each deployment environment.

## Notes
- Keep `config/` checked in with non-secret defaults; inject secrets via env vars or secret managers.
- Align values with the architecture docs (TTL caps, heartbeats, DB/Redis URLs).***

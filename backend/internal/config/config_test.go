package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromDir_MergesDefaultsEnvAndEnvVars(t *testing.T) {
	tmp := t.TempDir()

	common := `
app:
  env: local
  name: base
server:
  port: 8080
  max_request_body_bytes: 1024
  websocket:
    heartbeat_interval_seconds: 30
    idle_timeout_seconds: 60
database:
  url: postgres://base
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime_minutes: 30
redis:
  url: redis://base
  connection_pool_size: 5
messages:
  max_payload_bytes: 100
  max_inflight_per_connection: 3
live_location:
  max_ttl_seconds: 900
  strict_ttl: false
  publish_rate_per_minute: 120
logging:
  level: info
  json: false
`
	dev := `
database:
  url: postgres://dev
redis:
  url: redis://dev
messages:
  max_payload_bytes: 200
logging:
  json: true
`

	write(t, filepath.Join(tmp, "common"), "default-config.yaml", common)
	write(t, filepath.Join(tmp, "dev-qa"), "config.yaml", dev)

	t.Setenv("DB_URL", "postgres://override")
	t.Setenv("REDIS_URL", "")

	cfg, err := LoadFromDir(tmp, "dev-qa")
	if err != nil {
		t.Fatalf("LoadFromDir error: %v", err)
	}

	if got := cfg.App.Env; got != "dev-qa" {
		t.Fatalf("App.Env = %q, want dev-qa", got)
	}
	if got := cfg.Database.URL; got != "postgres://override" {
		t.Fatalf("Database.URL = %q, want override", got)
	}
	if got := cfg.Redis.URL; got != "redis://dev" {
		t.Fatalf("Redis.URL = %q, want redis://dev", got)
	}
	if got := cfg.Messages.MaxPayloadBytes; got != 200 {
		t.Fatalf("Messages.MaxPayloadBytes = %d, want 200", got)
	}
	if got := cfg.Logging.JSON; !got {
		t.Fatalf("Logging.JSON = false, want true")
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("Logging.Level = %q, want info", cfg.Logging.Level)
	}
	if cfg.LiveLocation.MaxTTLSeconds != 900 || cfg.LiveLocation.StrictTTL {
		t.Fatalf("LiveLocation defaults not preserved %+v", cfg.LiveLocation)
	}
}

func TestLoadFromDir_MissingDefaultsErrors(t *testing.T) {
	tmp := t.TempDir()
	if _, err := LoadFromDir(tmp, "local"); err == nil {
		t.Fatalf("expected error for missing defaults")
	}
}

func TestLoadFromDir_NoEnvFile_UsesDefaults(t *testing.T) {
	tmp := t.TempDir()
	common := `
app:
  env: local
  name: base
database:
  url: postgres://base
redis:
  url: redis://base
messages:
  max_payload_bytes: 100
live_location:
  max_ttl_seconds: 900
  strict_ttl: false
logging:
  level: info
  json: false
`
	write(t, filepath.Join(tmp, "common"), "default-config.yaml", common)

	cfg, err := LoadFromDir(tmp, "")
	if err != nil {
		t.Fatalf("LoadFromDir error: %v", err)
	}
	if cfg.App.Env != "local" {
		t.Fatalf("App.Env = %q, want local", cfg.App.Env)
	}
	if cfg.Database.URL != "postgres://base" {
		t.Fatalf("Database.URL = %q, want base", cfg.Database.URL)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigDir      = "config"
	commonDefaultsRelPath = "common/default-config.yaml"
	envConfigFileName     = "config.yaml"
)

// Config captures all runtime settings for the backend.
type Config struct {
	App struct {
		Env  string `yaml:"env"`
		Name string `yaml:"name"`
	} `yaml:"app"`
	Server struct {
		Port                int `yaml:"port"`
		MaxRequestBodyBytes int `yaml:"max_request_body_bytes"`
		Websocket           struct {
			HeartbeatIntervalSeconds int `yaml:"heartbeat_interval_seconds"`
			IdleTimeoutSeconds       int `yaml:"idle_timeout_seconds"`
		} `yaml:"websocket"`
	} `yaml:"server"`
	Database struct {
		URL                 string `yaml:"url"`
		MaxOpenConns        int    `yaml:"max_open_conns"`
		MaxIdleConns        int    `yaml:"max_idle_conns"`
		ConnMaxLifetimeMins int    `yaml:"conn_max_lifetime_minutes"`
	} `yaml:"database"`
	Redis struct {
		URL                string `yaml:"url"`
		ConnectionPoolSize int    `yaml:"connection_pool_size"`
	} `yaml:"redis"`
	Messages struct {
		MaxPayloadBytes          int `yaml:"max_payload_bytes"`
		MaxInFlightPerConnection int `yaml:"max_inflight_per_connection"`
	} `yaml:"messages"`
	LiveLocation struct {
		MaxTTLSeconds        int  `yaml:"max_ttl_seconds"`
		StrictTTL            bool `yaml:"strict_ttl"`
		PublishRatePerMinute int  `yaml:"publish_rate_per_minute"`
	} `yaml:"live_location"`
	Logging struct {
		Level string `yaml:"level"`
		JSON  bool   `yaml:"json"`
	} `yaml:"logging"`
}

// Load loads config using the default config directory.
func Load(env string) (*Config, error) {
	return LoadFromDir(defaultConfigDir, env)
}

// LoadFromDir loads config from a base directory containing common defaults and optional env overrides.
func LoadFromDir(baseDir, env string) (*Config, error) {
	if env == "" {
		env = "local"
	}

	cfg := &Config{}
	commonPath := filepath.Join(baseDir, commonDefaultsRelPath)
	if err := loadYAML(commonPath, cfg); err != nil {
		return nil, fmt.Errorf("load defaults: %w", err)
	}

	envPath := filepath.Join(baseDir, env, envConfigFileName)
	if err := loadIfExists(envPath, cfg); err != nil {
		return nil, fmt.Errorf("load env %q: %w", env, err)
	}

	cfg.App.Env = env
	applyEnvOverrides(cfg)
	return cfg, nil
}

func loadYAML(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func loadIfExists(path string, target interface{}) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return loadYAML(path, target)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DB_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("REDIS_URL"); v != "" {
		cfg.Redis.URL = v
	}
}

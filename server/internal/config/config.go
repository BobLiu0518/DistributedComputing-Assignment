package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL            string `json:"database_url"`
	RegistryAddr           string `json:"registry_addr"`
	Port                   int    `json:"port"`
	ServiceName            string `json:"service_name"`
	HeartbeatIntervalSec   int    `json:"heartbeat_interval_sec"`
	HeartbeatMaxFail       int    `json:"heartbeat_max_fail"`
	ReconnectBackoffSec    int    `json:"reconnect_backoff_sec"`
	ReconnectMaxBackoffSec int    `json:"reconnect_max_backoff_sec"`
}

func Load() *Config {
	cfg := defaultConfig()

	_ = loadFromFile(cfg)

	overrideFromEnv(cfg)
	return cfg
}

func defaultConfig() *Config {
	return &Config{
		DatabaseURL:            "postgres://postgres:postgres@localhost:5432/rpc?sslmode=disable",
		Port:                   8080,
		ServiceName:            "UserService",
		HeartbeatIntervalSec:   10,
		HeartbeatMaxFail:       2,
		ReconnectBackoffSec:    1,
		ReconnectMaxBackoffSec: 30,
	}
}

func loadFromFile(cfg *Config) error {
	_, b, _, _ := runtime.Caller(0)
	basePath := filepath.Dir(b)
	serverRoot := filepath.Join(basePath, "..", "..")

	paths := []string{
		filepath.Join(serverRoot, "server.json"),
		"server.json",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return json.Unmarshal(data, cfg)
	}

	return fmt.Errorf("config file not found")
}

func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("REGISTRY_ADDR"); v != "" {
		cfg.RegistryAddr = v
	}
	if v := os.Getenv("RPC_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := os.Getenv("SERVICE_NAME"); v != "" {
		cfg.ServiceName = v
	}
	if v := os.Getenv("HEARTBEAT_INTERVAL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.HeartbeatIntervalSec = n
		}
	}
	if v := os.Getenv("HEARTBEAT_MAX_FAIL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.HeartbeatMaxFail = n
		}
	}
	if v := os.Getenv("RECONNECT_BACKOFF_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ReconnectBackoffSec = n
		}
	}
	if v := os.Getenv("RECONNECT_MAX_BACKOFF_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ReconnectMaxBackoffSec = n
		}
	}
}

func (c *Config) HeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatIntervalSec) * time.Second
}

func (c *Config) ReconnectBackoff() time.Duration {
	return time.Duration(c.ReconnectBackoffSec) * time.Second
}

func (c *Config) ReconnectMaxBackoff() time.Duration {
	return time.Duration(c.ReconnectMaxBackoffSec) * time.Second
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("0.0.0.0:%d", c.Port)
}

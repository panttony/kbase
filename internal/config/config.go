package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
)

type Config struct {
	HTTPAddr      string
	PGDSN         string
	RedisAddr     string
	RedisPassword string
	IndexPath     string
	CacheTTL      time.Duration
	LogLevel      slog.Level
}

const (
	defaultHTTPAddr  = ":8080"
	defaultRedisAddr = "localhost:6379"
	defaultIndexPath = "./data/index"
	defaultCacheTTL  = 60 * time.Second
	defaultLogLevel  = slog.LevelInfo
)

func Load() (Config, error) {
	ttl, err := envDuration("CACHE_TTL", defaultCacheTTL)
	if err != nil {
		return Config{}, err
	}
	level, err := envLogLevel("LOG_LEVEL", defaultLogLevel)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:      envString("HTTP_ADDR", defaultHTTPAddr),
		PGDSN:         envString("PG_DSN", ""),
		RedisAddr:     envString("REDIS_ADDR", defaultRedisAddr),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		IndexPath:     envString("INDEX_PATH", defaultIndexPath),
		CacheTTL:      ttl,
		LogLevel:      level,
	}
	if cfg.PGDSN == "" {
		return Config{}, errors.New("PG_DSN is required")
	}
	return cfg, nil
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s must not be negative", key)
	}
	return d, nil
}

func envLogLevel(key string, fallback slog.Level) (slog.Level, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(v)); err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return lvl, nil
}

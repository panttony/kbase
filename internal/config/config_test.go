package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("INDEX_PATH", "")
	t.Setenv("CACHE_TTL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("PG_DSN", "postgres://u:p@host:5432/db?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want default :8080", cfg.HTTPAddr)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr = %q, want default", cfg.RedisAddr)
	}
	if cfg.IndexPath != "./data/index" {
		t.Errorf("IndexPath = %q, want default", cfg.IndexPath)
	}
	if cfg.CacheTTL != 60*time.Second {
		t.Errorf("CacheTTL = %v, want 60s", cfg.CacheTTL)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestLoadHonoursEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("REDIS_ADDR", "redis:6380")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("INDEX_PATH", "/tmp/idx")
	t.Setenv("CACHE_TTL", "5s")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("PG_DSN", "postgres://u:p@h/db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9999" || cfg.RedisAddr != "redis:6380" ||
		cfg.RedisPassword != "secret" || cfg.IndexPath != "/tmp/idx" {
		t.Errorf("env not honoured: %+v", cfg)
	}
	if cfg.CacheTTL != 5*time.Second {
		t.Errorf("CacheTTL = %v, want 5s", cfg.CacheTTL)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
}

func TestLoadRequiresPGDSN(t *testing.T) {
	t.Setenv("PG_DSN", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load without PG_DSN should fail")
	}
}

func TestLoadRejectsBadCacheTTL(t *testing.T) {
	t.Setenv("CACHE_TTL", "not-a-duration")
	t.Setenv("PG_DSN", "postgres://u:p@h/db")
	if _, err := Load(); err == nil {
		t.Fatal("Load with bad CACHE_TTL should fail")
	}
}

package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("TEST_PG_DSN"); v != "" {
		return v
	}
	return "postgres://kbase:kbase@localhost:5432/kbase?sslmode=disable"
}

func newTestRepo(t *testing.T) *RepoPostgres {
	t.Helper()

	probe, err := pgxpool.New(context.Background(), dsn())
	if err != nil {
		t.Skipf("postgres unavailable (set TEST_PG_DSN or start kbase-test-pg): %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	pingErr := probe.Ping(ctx)
	cancel()
	probe.Close()
	if pingErr != nil {
		t.Skipf("postgres unavailable (set TEST_PG_DSN or start kbase-test-pg): %v", pingErr)
	}

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	admin, err := pgxpool.New(context.Background(), dsn())
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer admin.Close()

	ctx = context.Background()
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema %s: %v", schema, err)
	}

	cfg, err := pgxpool.ParseConfig(dsn())
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 5
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect schema pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	repo, err := New(ctx, pool)
	if err != nil {
		t.Fatalf("New repo: %v", err)
	}
	return repo
}

package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func addr() string {
	if v := os.Getenv("TEST_REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6379"
}

func newTestCache(t *testing.T) *RedisCache {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	probe := redis.NewClient(&redis.Options{Addr: addr()})
	if err := probe.Ping(ctx).Err(); err != nil {
		probe.Close()
		t.Skipf("redis unavailable (set TEST_REDIS_ADDR or start kbase-test-redis): %v", err)
	}
	probe.Close()

	prefix := "search:v1:test" + fmt.Sprintf("%d", time.Now().UnixNano()) + ":"
	return New(addr(), "", prefix)
}

func TestCacheSetGetTTLAndInvalidation(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()
	ctx := context.Background()

	key1 := "abc"
	key2 := "def"

	if v, err := c.Get(ctx, key1); err != nil || v != "" {
		t.Fatalf("Get(miss) = %q, %v; want empty, nil", v, err)
	}

	if err := c.Set(ctx, key1, `{"hits":[]}`, 10*time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Set(ctx, key2, `{"hits":[]}`, 10*time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if v, err := c.Get(ctx, key1); err != nil || v != `{"hits":[]}` {
		t.Fatalf("Get = %q, %v; want cached value", v, err)
	}

	if err := c.InvalidateAll(ctx, ""); err != nil {
		t.Fatalf("InvalidateAll: %v", err)
	}
	if v, _ := c.Get(ctx, key1); v != "" {
		t.Errorf("key1 not invalidated: %q", v)
	}
	if v, _ := c.Get(ctx, key2); v != "" {
		t.Errorf("key2 not invalidated: %q", v)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()
	ctx := context.Background()

	key := "ttl"
	if err := c.Set(ctx, key, "value", 1*time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if v, _ := c.Get(ctx, key); v != "" {
		t.Errorf("entry should have expired, got %q", v)
	}
}

func TestCacheKeyIsolation(t *testing.T) {
	a := newTestCache(t)
	defer a.Close()
	b := newTestCache(t) // different prefix => different namespace
	defer b.Close()
	ctx := context.Background()

	if err := a.Set(ctx, "shared", "from-a", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if v, _ := b.Get(ctx, "shared"); v != "" {
		t.Errorf("cache b saw cache a's value %q; prefixes must isolate", v)
	}
}

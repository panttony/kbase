package cache

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	InvalidateAll(ctx context.Context, prefix string) error
	Close() error
}

type RedisCache struct {
	client *redis.Client
	prefix string
}

func New(addr, password string, prefix string) *RedisCache {
	return &RedisCache{
		client: redis.NewClient(&redis.Options{Addr: addr, Password: password}),
		prefix: prefix,
	}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, c.prefix+key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		log.Printf("cache get %s: %v", key, err)
		return "", nil
	}
	return val, nil
}

func (c *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := c.client.Set(ctx, c.prefix+key, value, ttl).Err(); err != nil {
		log.Printf("cache set %s: %v", key, err)
		return err
	}
	return nil
}

func (c *RedisCache) InvalidateAll(ctx context.Context, prefix string) error {
	if prefix == "" {
		prefix = c.prefix
	}
	var keys []string
	iter := c.client.Scan(ctx, 0, prefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	err := iter.Err()
	if err != nil {
		log.Printf("cache scan %s: %v", prefix, err)
		return err
	}
	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			log.Printf("cache del %d keys: %v", len(keys), err)
			return err
		}
	}
	return nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

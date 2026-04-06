package cache

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	ctx    context.Context
}

func New(addr, password string) *Cache {
	if addr == "" {
		log.Println("Redis address not configured, caching disabled")
		return &Cache{ctx: context.Background()}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Redis not reachable at %s, caching disabled: %v", addr, err)
		return &Cache{ctx: ctx}
	}

	log.Printf("Redis connected at %s", addr)
	return &Cache{client: client, ctx: ctx}
}

func (c *Cache) IsConnected() bool {
	if c.client == nil {
		return false
	}
	return c.client.Ping(c.ctx).Err() == nil
}

func (c *Cache) Get(key string) (string, bool) {
	if c.client == nil {
		return "", false
	}
	val, err := c.client.Get(c.ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

func (c *Cache) Set(key, value string, ttl time.Duration) {
	if c.client == nil {
		return
	}
	c.client.Set(c.ctx, key, value, ttl)
}

func (c *Cache) Delete(key string) {
	if c.client == nil {
		return
	}
	c.client.Del(c.ctx, key)
}

func (c *Cache) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

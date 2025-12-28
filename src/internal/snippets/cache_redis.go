package snippets

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	prefix string
}

func NewRedisCache(client *redis.Client, prefix string) *RedisCache {
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "sniply:cache:"
	}
	return &RedisCache{client: client, prefix: p}
}

func (c *RedisCache) keyByID(id string) string {
	return c.prefix + "snippet:" + id
}

func (c *RedisCache) keyList(key string) string {
	return c.prefix + "snippet:list:" + key
}

func (c *RedisCache) GetByID(ctx context.Context, id string) (*Snippet, bool, error) {
	val, err := c.client.Get(ctx, c.keyByID(id)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var s Snippet
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return nil, false, err
	}
	return &s, true, nil
}

func (c *RedisCache) SetByID(ctx context.Context, s *Snippet, ttl time.Duration) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.keyByID(s.ID), payload, ttl).Err()
}

func (c *RedisCache) DeleteByID(ctx context.Context, id string) error {
	return c.client.Del(ctx, c.keyByID(id)).Err()
}

func (c *RedisCache) GetList(ctx context.Context, key string) ([]*Snippet, bool, error) {
	val, err := c.client.Get(ctx, c.keyList(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var out []*Snippet
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func (c *RedisCache) SetList(ctx context.Context, key string, snippets []*Snippet, ttl time.Duration) error {
	payload, err := json.Marshal(snippets)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.keyList(key), payload, ttl).Err()
}

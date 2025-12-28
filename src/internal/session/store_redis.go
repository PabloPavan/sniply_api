package session

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "sniply:session:"
	}
	return &RedisStore{client: client, prefix: p}
}

func (s *RedisStore) key(id string) string {
	return s.prefix + id
}

func (s *RedisStore) Set(ctx context.Context, id string, sess Session, ttl time.Duration) error {
	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(id), payload, ttl).Err()
}

func (s *RedisStore) Get(ctx context.Context, id string) (*Session, error) {
	val, err := s.client.Get(ctx, s.key(id)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var sess Session
	if err := json.Unmarshal([]byte(val), &sess); err != nil {
		return nil, err
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.client.Del(ctx, s.key(id)).Err()
		return nil, ErrNotFound
	}
	return &sess, nil
}

func (s *RedisStore) Delete(ctx context.Context, id string) error {
	return s.client.Del(ctx, s.key(id)).Err()
}

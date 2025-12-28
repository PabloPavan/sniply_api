package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	Client *redis.Client
	Prefix string
	Limit  int
	Window time.Duration
}

var allowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
if current > tonumber(ARGV[1]) then
  local ttl = redis.call("PTTL", KEYS[1])
  return {0, ttl}
end
local ttl = redis.call("PTTL", KEYS[1])
return {1, ttl}
`)

func (l *Limiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	if l.Client == nil {
		return true, 0, nil
	}

	limit := l.Limit
	if limit <= 0 {
		limit = 5
	}
	window := l.Window
	if window <= 0 {
		window = time.Minute
	}

	fullKey := l.Prefix + key
	res, err := allowScript.Run(ctx, l.Client, []string{fullKey}, limit, window.Milliseconds()).Result()
	if err != nil {
		return false, 0, err
	}

	values, ok := res.([]any)
	if !ok || len(values) != 2 {
		return false, 0, redis.ErrClosed
	}

	allowed, _ := values[0].(int64)
	ttlMs, _ := values[1].(int64)

	return allowed == 1, time.Duration(ttlMs) * time.Millisecond, nil
}

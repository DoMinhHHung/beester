package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var tokenBucketScript = redis.NewScript(`
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local cost = tonumber(ARGV[3])

local now_parts = redis.call('TIME')
local now = tonumber(now_parts[1]) * 1000 + math.floor(tonumber(now_parts[2]) / 1000)

local values = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(values[1])
local ts = tonumber(values[2])

if tokens == nil then tokens = capacity end
if ts == nil then ts = now end

local delta = math.max(0, now - ts) / 1000.0
tokens = math.min(capacity, tokens + delta * refill)

local allowed = 0
local retry_ms = 0
if tokens >= cost then
  allowed = 1
  tokens = tokens - cost
else
  retry_ms = math.ceil((cost - tokens) / refill * 1000)
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
local ttl_ms = math.max(1000, math.ceil((capacity / refill) * 2 * 1000))
redis.call('PEXPIRE', KEYS[1], ttl_ms)

return {allowed, math.floor(tokens), retry_ms}
`)

type RedisLimiter struct {
	client          *redis.Client
	capacity        int
	refillPerSecond float64
	keyPrefix       string
}

func NewRedisLimiter(
	client *redis.Client,
	capacity int,
	refillPerSecond float64,
	keyPrefix string,
) (*RedisLimiter, error) {
	if client == nil {
		return nil, errors.New("Redis client is required")
	}
	if capacity <= 0 {
		return nil, errors.New("rate limit capacity must be > 0")
	}
	if refillPerSecond <= 0 {
		return nil, errors.New("rate limit refill per second must be > 0")
	}
	keyPrefix = strings.TrimSpace(keyPrefix)
	if keyPrefix == "" {
		return nil, errors.New("rate limit key prefix is required")
	}

	return &RedisLimiter{
		client:          client,
		capacity:        capacity,
		refillPerSecond: refillPerSecond,
		keyPrefix:       keyPrefix,
	}, nil
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) (Decision, error) {
	if l == nil || l.client == nil {
		return Decision{}, errors.New("Redis limiter is not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return Decision{}, errors.New("rate limit key is required")
	}

	result, err := tokenBucketScript.Run(
		ctx,
		l.client,
		[]string{l.redisKey(key)},
		l.capacity,
		l.refillPerSecond,
		1,
	).Result()
	if err != nil {
		return Decision{}, fmt.Errorf("execute Redis token bucket: %w", err)
	}

	values, ok := result.([]any)
	if !ok || len(values) != 3 {
		return Decision{}, fmt.Errorf("unexpected Redis token bucket result %#v", result)
	}

	allowed, err := redisInt64(values[0])
	if err != nil {
		return Decision{}, fmt.Errorf("parse token bucket allowed: %w", err)
	}
	remaining, err := redisInt64(values[1])
	if err != nil {
		return Decision{}, fmt.Errorf("parse token bucket remaining: %w", err)
	}
	retryMS, err := redisInt64(values[2])
	if err != nil {
		return Decision{}, fmt.Errorf("parse token bucket retry: %w", err)
	}

	return Decision{
		Allowed:    allowed == 1,
		Limit:      l.capacity,
		Remaining:  max(0, int(remaining)),
		RetryAfter: time.Duration(max(int64(0), retryMS)) * time.Millisecond,
	}, nil
}

func (l *RedisLimiter) redisKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return l.keyPrefix + ":" + hex.EncodeToString(sum[:])
}

func redisInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case float64:
		if typed > math.MaxInt64 || typed < math.MinInt64 {
			return 0, fmt.Errorf("number %v is outside int64 range", typed)
		}
		return int64(typed), nil
	case string:
		var result int64
		_, err := fmt.Sscan(typed, &result)
		return result, err
	case []byte:
		var result int64
		_, err := fmt.Sscan(string(typed), &result)
		return result, err
	default:
		return 0, fmt.Errorf("unsupported Redis number type %T", value)
	}
}

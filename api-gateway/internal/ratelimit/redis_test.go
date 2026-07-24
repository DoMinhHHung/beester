package ratelimit

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisLimiterTokenBucket(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	defer func() { _ = client.Close() }()
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	limiter, err := NewRedisLimiter(client, 2, 0.01, "test:"+uuid.Must(uuid.NewV7()).String())
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil || !first.Allowed {
		t.Fatalf("expected first request allowed, decision=%#v err=%v", first, err)
	}
	second, err := limiter.Allow(ctx, "user:1")
	if err != nil || !second.Allowed {
		t.Fatalf("expected second request allowed, decision=%#v err=%v", second, err)
	}
	third, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("third request: %v", err)
	}
	if third.Allowed {
		t.Fatalf("expected third request denied, decision=%#v", third)
	}
	if third.RetryAfter <= 0 {
		t.Fatalf("expected positive retry delay, got %s", third.RetryAfter)
	}
}

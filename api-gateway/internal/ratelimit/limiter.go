package ratelimit

import (
	"context"
	"time"
)

type Decision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}

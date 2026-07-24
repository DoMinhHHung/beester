package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
	"github.com/DoMinhHHung/beester/api-gateway/internal/ratelimit"
	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
)

func RateLimit(
	logger *slog.Logger,
	limiter ratelimit.Limiter,
	failOpen bool,
	trustProxy bool,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rateLimitKey(r, trustProxy)
		decision, err := limiter.Allow(r.Context(), key)
		if err != nil {
			logger.ErrorContext(
				r.Context(),
				"rate limiter failed",
				slog.String("request_id", requestid.FromContext(r.Context())),
				slog.Any("error", err),
			)
			if failOpen {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		setRateLimitHeaders(w, decision)
		if !decision.Allowed {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func rateLimitKey(r *http.Request, trustProxy bool) string {
	if value, ok := identity.FromContext(r.Context()); ok && value.UserID != "" {
		return "user:" + value.UserID
	}
	return "ip:" + clientIP(r, trustProxy)
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			if first, _, found := strings.Cut(forwarded, ","); found {
				return strings.TrimSpace(first)
			}
			return forwarded
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func setRateLimitHeaders(w http.ResponseWriter, decision ratelimit.Decision) {
	w.Header().Set("RateLimit-Limit", strconv.Itoa(decision.Limit))
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(max(0, decision.Remaining)))
	if !decision.Allowed && decision.RetryAfter > 0 {
		seconds := int64((decision.RetryAfter + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.FormatInt(max(int64(1), seconds), 10))
	}
}

// RateLimitKeyFromContext is exposed for gRPC middleware so HTTP and gRPC use the
// same user-first keying rule without duplicating identity semantics.
func RateLimitKeyFromContext(ctx context.Context, fallback string) string {
	if value, ok := identity.FromContext(ctx); ok && value.UserID != "" {
		return "user:" + value.UserID
	}
	return fmt.Sprintf("ip:%s", fallback)
}

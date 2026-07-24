package grpcmiddleware

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
	"github.com/DoMinhHHung/beester/api-gateway/internal/ratelimit"
	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func RateLimitStreamInterceptor(
	logger *slog.Logger,
	limiter ratelimit.Limiter,
	failOpen bool,
) grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		key := grpcRateLimitKey(stream)
		decision, err := limiter.Allow(stream.Context(), key)
		if err != nil {
			logger.ErrorContext(
				stream.Context(),
				"gRPC rate limiter failed",
				slog.String("request_id", requestid.FromContext(stream.Context())),
				slog.Any("error", err),
			)
			if failOpen {
				return handler(srv, stream)
			}
			return status.Error(codes.Unavailable, "rate limiter unavailable")
		}

		headers := metadata.Pairs(
			"ratelimit-limit", strconv.Itoa(decision.Limit),
			"ratelimit-remaining", strconv.Itoa(max(0, decision.Remaining)),
		)
		if !decision.Allowed && decision.RetryAfter > 0 {
			seconds := int64((decision.RetryAfter + time.Second - 1) / time.Second)
			headers.Append("retry-after", strconv.FormatInt(max(int64(1), seconds), 10))
		}
		_ = stream.SetHeader(headers)

		if !decision.Allowed {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(srv, stream)
	}
}

func grpcRateLimitKey(stream grpc.ServerStream) string {
	if value, ok := identity.FromContext(stream.Context()); ok && value.UserID != "" {
		return "user:" + value.UserID
	}
	if value, ok := peer.FromContext(stream.Context()); ok && value.Addr != nil {
		address := value.Addr.String()
		if host, _, err := net.SplitHostPort(address); err == nil && host != "" {
			address = host
		}
		return "ip:" + address
	}
	return fmt.Sprintf("ip:unknown:%s", requestid.FromContext(stream.Context()))
}

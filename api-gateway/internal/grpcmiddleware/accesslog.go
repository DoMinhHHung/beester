package grpcmiddleware

import (
	"log/slog"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func AccessLogStreamInterceptor(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, stream)

		clientAddr := ""
		if value, ok := peer.FromContext(stream.Context()); ok && value.Addr != nil {
			clientAddr = value.Addr.String()
		}

		logger.LogAttrs(
			stream.Context(),
			slog.LevelInfo,
			"gRPC request completed",
			slog.String("request_id", requestid.FromContext(stream.Context())),
			slog.String("method", info.FullMethod),
			slog.String("client_addr", clientAddr),
			slog.String("code", status.Code(err).String()),
			slog.Duration("duration", time.Since(start)),
		)
		return err
	}
}

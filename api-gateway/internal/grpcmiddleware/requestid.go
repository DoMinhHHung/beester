package grpcmiddleware

import (
	"context"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func RequestIDStreamInterceptor(
	srv any,
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	id := incomingRequestID(stream.Context())
	if id == "" {
		id = uuid.Must(uuid.NewV7()).String()
	}

	_ = stream.SetHeader(metadata.Pairs(requestid.MetadataKey, id))
	ctx := requestid.WithContext(stream.Context(), id)
	return handler(srv, &wrappedServerStream{ServerStream: stream, ctx: ctx})
}

func incomingRequestID(ctx context.Context) string {
	values := metadata.ValueFromIncomingContext(ctx, requestid.MetadataKey)
	if len(values) == 0 || values[0] == "" {
		return ""
	}
	if _, err := uuid.Parse(values[0]); err != nil {
		return ""
	}
	return values[0]
}

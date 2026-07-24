package grpcmiddleware

import (
	"strings"

	"github.com/DoMinhHHung/beester/api-gateway/internal/auth"
	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func JWTAuthStreamInterceptor(
	validator *auth.Validator,
	publicMethodPrefixes []string,
) grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if methodIsPublic(info.FullMethod, publicMethodPrefixes) {
			return handler(srv, stream)
		}

		values := metadata.ValueFromIncomingContext(stream.Context(), "authorization")
		if len(values) == 0 {
			return status.Error(codes.Unauthenticated, "authorization is required")
		}

		tokenString, ok := grpcBearerToken(values[0])
		if !ok {
			return status.Error(codes.Unauthenticated, "invalid authorization metadata")
		}
		userID, err := validator.Validate(tokenString)
		if err != nil {
			return status.Error(codes.Unauthenticated, "invalid access token")
		}

		ctx := identity.WithContext(stream.Context(), identity.Identity{UserID: userID})
		return handler(srv, &wrappedServerStream{ServerStream: stream, ctx: ctx})
	}
}

func grpcBearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func methodIsPublic(method string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return false
}

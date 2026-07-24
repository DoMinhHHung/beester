package grpcclient

import (
	"context"
	"testing"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUnaryRequestIDInterceptor(t *testing.T) {
	id := uuid.Must(uuid.NewV7()).String()

	ctx := metadata.AppendToOutgoingContext(
		context.Background(),
		"authorization",
		"Bearer test-token",
	)

	ctx = requestid.WithContext(ctx, id)

	invoker := func(
		ctx context.Context,
		_ string,
		_ any,
		_ any,
		_ *grpc.ClientConn,
		_ ...grpc.CallOption,
	) error {
		assertMetadataValue(
			t,
			ctx,
			requestid.MetadataKey,
			id,
		)

		assertMetadataValue(
			t,
			ctx,
			"authorization",
			"Bearer test-token",
		)

		return nil
	}

	err := UnaryRequestIDInterceptor(
		ctx,
		"/test.Service/Get",
		nil,
		nil,
		nil,
		invoker,
	)
	if err != nil {
		t.Fatalf("invoke unary interceptor: %v", err)
	}
}

func TestStreamRequestIDInterceptor(t *testing.T) {
	id := uuid.Must(uuid.NewV7()).String()

	ctx := requestid.WithContext(
		context.Background(),
		id,
	)

	streamer := func(
		ctx context.Context,
		_ *grpc.StreamDesc,
		_ *grpc.ClientConn,
		_ string,
		_ ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		assertMetadataValue(
			t,
			ctx,
			requestid.MetadataKey,
			id,
		)

		return nil, nil
	}

	_, err := StreamRequestIDInterceptor(
		ctx,
		&grpc.StreamDesc{},
		nil,
		"/test.Service/Watch",
		streamer,
	)
	if err != nil {
		t.Fatalf("invoke stream interceptor: %v", err)
	}
}

func TestUnaryRequestIDInterceptorWithoutRequestID(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(
		context.Background(),
		"authorization",
		"Bearer test-token",
	)

	invoker := func(
		ctx context.Context,
		_ string,
		_ any,
		_ any,
		_ *grpc.ClientConn,
		_ ...grpc.CallOption,
	) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Fatal("expected outgoing metadata")
		}

		if values := md.Get(requestid.MetadataKey); len(values) != 0 {
			t.Fatalf(
				"expected no request ID metadata, got %v",
				values,
			)
		}

		assertMetadataValue(
			t,
			ctx,
			"authorization",
			"Bearer test-token",
		)

		return nil
	}

	err := UnaryRequestIDInterceptor(
		ctx,
		"/test.Service/Get",
		nil,
		nil,
		nil,
		invoker,
	)
	if err != nil {
		t.Fatalf("invoke unary interceptor: %v", err)
	}
}

func assertMetadataValue(
	t *testing.T,
	ctx context.Context,
	key string,
	want string,
) {
	t.Helper()

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}

	values := md.Get(key)

	if len(values) != 1 {
		t.Fatalf(
			"expected exactly one value for metadata %q, got %v",
			key,
			values,
		)
	}

	if got := values[0]; got != want {
		t.Fatalf(
			"expected metadata %q=%q, got %q",
			key,
			want,
			got,
		)
	}
}

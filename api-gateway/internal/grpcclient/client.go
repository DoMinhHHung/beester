package grpcclient

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func New(
	target string,
	transportCredentials credentials.TransportCredentials,
) (*grpc.ClientConn, error) {
	if target == "" {
		return nil, fmt.Errorf("gRPC target is required")
	}

	if transportCredentials == nil {
		return nil, fmt.Errorf("gRPC transport credentials are required")
	}

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithChainUnaryInterceptor(
			UnaryRequestIDInterceptor,
		),
		grpc.WithChainStreamInterceptor(
			StreamRequestIDInterceptor,
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create gRPC client for %q: %w",
			target,
			err,
		)
	}

	return conn, nil
}

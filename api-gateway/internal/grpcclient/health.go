package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func CheckHealth(
	ctx context.Context,
	conn grpc.ClientConnInterface,
	service string,
) error {
	if conn == nil {
		return fmt.Errorf("gRPC connection is required")
	}

	client := healthpb.NewHealthClient(conn)

	response, err := client.Check(
		ctx,
		&healthpb.HealthCheckRequest{
			Service: service,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"check gRPC health for service %q: %w",
			service,
			err,
		)
	}

	if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf(
			"gRPC service %q is not serving: %s",
			service,
			response.GetStatus(),
		)
	}

	return nil
}

package grpcproxy

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcrouting"
	"github.com/DoMinhHHung/beester/api-gateway/internal/upstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestTransparentProxyForwardsUnaryHealthRPC(t *testing.T) {
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen backend: %v", err)
	}
	backendServer := grpc.NewServer()
	backendHealth := health.NewServer()
	backendHealth.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(backendServer, backendHealth)
	go func() { _ = backendServer.Serve(backendListener) }()
	defer backendServer.Stop()

	registry, err := upstream.New(
		[]upstream.Spec{{Name: "backend", Target: backendListener.Addr().String()}},
		insecure.NewCredentials(),
	)
	if err != nil {
		t.Fatalf("create upstream registry: %v", err)
	}
	defer func() { _ = registry.Close() }()

	routes, err := grpcrouting.New([]grpcrouting.Route{{
		Prefix: "/grpc.health.v1.Health/", Upstream: "backend",
	}})
	if err != nil {
		t.Fatalf("create route table: %v", err)
	}

	proxy := New(routes, registry, "x-user-id", true)
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	proxyServer := grpc.NewServer(
		grpc.UnknownServiceHandler(proxy.Handle),
		grpc.ForceServerCodec(Codec()),
	)
	go func() { _ = proxyServer.Serve(proxyListener) }()
	defer proxyServer.Stop()

	conn, err := grpc.NewClient(
		proxyListener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check through proxy: %v", err)
	}
	if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("expected SERVING, got %s", response.GetStatus())
	}
}

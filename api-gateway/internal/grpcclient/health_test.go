package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type fakeClientConn struct {
	grpc.ClientConnInterface

	status healthpb.HealthCheckResponse_ServingStatus
	err    error
}

func (f *fakeClientConn) Invoke(
	_ context.Context,
	_ string,
	_ any,
	reply any,
	_ ...grpc.CallOption,
) error {
	if f.err != nil {
		return f.err
	}

	response, ok := reply.(*healthpb.HealthCheckResponse)
	if !ok {
		return errors.New("unexpected response type")
	}

	response.Status = f.status

	return nil
}

func TestCheckHealthServing(t *testing.T) {
	conn := &fakeClientConn{
		status: healthpb.HealthCheckResponse_SERVING,
	}

	err := CheckHealth(
		context.Background(),
		conn,
		"",
	)
	if err != nil {
		t.Fatalf(
			"expected healthy service, got %v",
			err,
		)
	}
}

func TestCheckHealthNotServing(t *testing.T) {
	conn := &fakeClientConn{
		status: healthpb.HealthCheckResponse_NOT_SERVING,
	}

	err := CheckHealth(
		context.Background(),
		conn,
		"",
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckHealthRPCError(t *testing.T) {
	expectedErr := errors.New("transport unavailable")

	conn := &fakeClientConn{
		err: expectedErr,
	}

	err := CheckHealth(
		context.Background(),
		conn,
		"",
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected wrapped error %v, got %v",
			expectedErr,
			err,
		)
	}
}

func TestCheckHealthRequiresConnection(t *testing.T) {
	err := CheckHealth(
		context.Background(),
		nil,
		"",
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

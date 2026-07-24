package grpcclient

import (
	"testing"

	"google.golang.org/grpc/credentials/insecure"
)

func TestNewRequiresTarget(t *testing.T) {
	conn, err := New(
		"",
		insecure.NewCredentials(),
	)

	if err == nil {
		if conn != nil {
			_ = conn.Close()
		}

		t.Fatal("expected error, got nil")
	}

	if conn != nil {
		_ = conn.Close()

		t.Fatal("expected nil connection")
	}
}

func TestNewRequiresTransportCredentials(t *testing.T) {
	conn, err := New(
		"localhost:50051",
		nil,
	)

	if err == nil {
		if conn != nil {
			_ = conn.Close()
		}

		t.Fatal("expected error, got nil")
	}

	if conn != nil {
		_ = conn.Close()

		t.Fatal("expected nil connection")
	}
}

func TestNewCreatesClientConnection(t *testing.T) {
	conn, err := New(
		"localhost:50051",
		insecure.NewCredentials(),
	)
	if err != nil {
		t.Fatalf("create gRPC client: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close gRPC client: %v", err)
		}
	}()

	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
}

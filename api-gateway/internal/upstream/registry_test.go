package upstream

import (
	"testing"

	"google.golang.org/grpc/credentials/insecure"
)

func TestNewAllowsEmptyRegistry(t *testing.T) {
	registry, err := New(nil, nil)
	if err != nil {
		t.Fatalf("create empty registry: %v", err)
	}
	defer func() {
		if err := registry.Close(); err != nil {
			t.Errorf("close registry: %v", err)
		}
	}()

	if got := len(registry.Names()); got != 0 {
		t.Fatalf(
			"expected no upstreams, got %d",
			got,
		)
	}
}

func TestRegistryCreatesAndLooksUpConnections(t *testing.T) {
	registry, err := New(
		[]Spec{
			{
				Name:   "auth",
				Target: "localhost:50051",
			},
			{
				Name:   "user",
				Target: "localhost:50052",
			},
		},
		insecure.NewCredentials(),
	)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	defer func() {
		if err := registry.Close(); err != nil {
			t.Errorf("close registry: %v", err)
		}
	}()

	names := registry.Names()

	if len(names) != 2 {
		t.Fatalf(
			"expected 2 upstreams, got %d",
			len(names),
		)
	}

	conn, ok := registry.Conn("auth")
	if !ok {
		t.Fatal("expected auth upstream")
	}

	if conn == nil {
		t.Fatal("expected non-nil auth connection")
	}

	if _, ok := registry.Conn("missing"); ok {
		t.Fatal("expected missing upstream not to exist")
	}
}

func TestRegistryRejectsDuplicateNames(t *testing.T) {
	_, err := New(
		[]Spec{
			{
				Name:   "auth",
				Target: "localhost:50051",
			},
			{
				Name:   "auth",
				Target: "localhost:50052",
			},
		},
		insecure.NewCredentials(),
	)

	if err == nil {
		t.Fatal("expected duplicate upstream error")
	}
}

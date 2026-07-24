package readiness

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCheckerReturnsNilWhenAllChecksPass(t *testing.T) {
	checker, err := New(
		Check{
			Name: "auth-service",
			Run: func(context.Context) error {
				return nil
			},
		},
		Check{
			Name: "user-service",
			Run: func(context.Context) error {
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("create checker: %v", err)
	}

	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf(
			"expected readiness success, got %v",
			err,
		)
	}
}

func TestCheckerReturnsAllFailures(t *testing.T) {
	checker, err := New(
		Check{
			Name: "auth-service",
			Run: func(context.Context) error {
				return errors.New("unavailable")
			},
		},
		Check{
			Name: "user-service",
			Run: func(context.Context) error {
				return errors.New("timeout")
			},
		},
	)
	if err != nil {
		t.Fatalf("create checker: %v", err)
	}

	err = checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected readiness failure")
	}

	message := err.Error()

	for _, expected := range []string{
		"auth-service",
		"user-service",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf(
				"expected error to contain %q, got %q",
				expected,
				message,
			)
		}
	}
}

func TestNewRejectsDuplicateNames(t *testing.T) {
	_, err := New(
		Check{
			Name: "auth-service",
			Run: func(context.Context) error {
				return nil
			},
		},
		Check{
			Name: "auth-service",
			Run: func(context.Context) error {
				return nil
			},
		},
	)

	if err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestNewAllowsNoChecks(t *testing.T) {
	checker, err := New()
	if err != nil {
		t.Fatalf("create empty checker: %v", err)
	}

	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf(
			"expected empty checker to be ready, got %v",
			err,
		)
	}
}

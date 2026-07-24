package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(
		logger,
		func(context.Context) error {
			return nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			rec.Code,
		)
	}

	if got, want := rec.Body.String(), "ok\n"; got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}

	if got, want := rec.Header().Get("Content-Type"), "text/plain; charset=utf-8"; got != want {
		t.Fatalf(
			"expected Content-Type %q, got %q",
			want,
			got,
		)
	}
}

func TestReadyzReturnsOKWhenReady(t *testing.T) {
	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := NewHandler(
		logger,
		func(context.Context) error {
			return nil
		},
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/readyz",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			rec.Code,
		)
	}

	if got, want := rec.Body.String(), "ready\n"; got != want {
		t.Fatalf(
			"expected body %q, got %q",
			want,
			got,
		)
	}
}

func TestReadyzReturnsServiceUnavailableWhenNotReady(t *testing.T) {
	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := NewHandler(
		logger,
		func(context.Context) error {
			return errors.New("dependency unavailable")
		},
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/readyz",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			rec.Code,
		)
	}
}

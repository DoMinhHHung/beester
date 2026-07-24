package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"github.com/DoMinhHHung/beester/api-gateway/internal/routing"
	"github.com/DoMinhHHung/beester/api-gateway/internal/upstream"
)

func TestHTTPDispatcherProxiesAndInjectsTrustedHeaders(t *testing.T) {
	var gotRequestID string
	var gotUserID string
	var gotAuthorization string
	var gotPath string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = r.Header.Get(requestid.Header)
		gotUserID = r.Header.Get("X-User-ID")
		gotAuthorization = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set(requestid.Header, "00000000-0000-4000-8000-000000000000")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := upstream.NewHTTP(
		[]upstream.HTTPSpec{{Name: "users", Target: backend.URL}},
		logger,
		"X-User-ID",
		false,
	)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	defer registry.CloseIdleConnections()

	table, err := routing.New([]routing.Route{{Method: http.MethodGet, Pattern: "/api/users/{id}", Upstream: "users"}})
	if err != nil {
		t.Fatalf("create route table: %v", err)
	}

	handler := NewHTTPDispatcher(table, registry, logger, time.Second)
	req := httptest.NewRequest(http.MethodGet, "http://gateway/api/users/123", nil)
	req.Header.Set("Authorization", "Bearer client-token")
	req.Header.Set("X-User-ID", "spoofed")
	ctx := requestid.WithContext(req.Context(), "019f937e-0aca-7abf-b643-b35f6cfaa9d1")
	ctx = identity.WithContext(ctx, identity.Identity{UserID: "user-123"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d body=%q", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if gotRequestID == "" {
		t.Fatal("expected request ID upstream header")
	}
	if rec.Header().Get(requestid.Header) != gotRequestID {
		t.Fatalf("expected gateway request ID in response, got %q", rec.Header().Get(requestid.Header))
	}
	if gotUserID != "user-123" {
		t.Fatalf("expected trusted user id, got %q", gotUserID)
	}
	if gotAuthorization != "" {
		t.Fatalf("expected authorization stripped, got %q", gotAuthorization)
	}
	if gotPath != "/api/users/123" {
		t.Fatalf("expected original path, got %q", gotPath)
	}
}

func TestHTTPDispatcherReturnsGatewayTimeout(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := upstream.NewHTTP([]upstream.HTTPSpec{{Name: "slow", Target: backend.URL}}, logger, "X-User-ID", false)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	defer registry.CloseIdleConnections()

	table, err := routing.New([]routing.Route{{Method: http.MethodGet, Pattern: "/slow", Upstream: "slow"}})
	if err != nil {
		t.Fatalf("create route table: %v", err)
	}

	handler := NewHTTPDispatcher(table, registry, logger, 10*time.Millisecond)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://gateway/slow", nil))
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected %d, got %d", http.StatusGatewayTimeout, rec.Code)
	}
}

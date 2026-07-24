package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/auth"
	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
	"github.com/DoMinhHHung/beester/api-gateway/internal/ratelimit"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTAuthRejectsMissingTokenAndInjectsIdentity(t *testing.T) {
	validator, err := auth.NewHMACValidator("secret", "", "", "sub", 0)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, ok := identity.FromContext(r.Context())
		if ok {
			gotUserID = value.UserID
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := JWTAuth(validator, []string{"/public/"}, "X-User-ID", next)

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/private", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", missing.Code)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-42",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	req.Header.Set("X-User-ID", "spoofed")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if gotUserID != "user-42" {
		t.Fatalf("expected user-42, got %q", gotUserID)
	}
}

type fakeLimiter struct {
	decision ratelimit.Decision
	err      error
	key      string
}

func (f *fakeLimiter) Allow(_ context.Context, key string) (ratelimit.Decision, error) {
	f.key = key
	return f.decision, f.err
}

func TestRateLimitUsesUserIDAndRejectsExceeded(t *testing.T) {
	limiter := &fakeLimiter{decision: ratelimit.Decision{
		Allowed: false, Limit: 10, Remaining: 0, RetryAfter: 1500 * time.Millisecond,
	}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	})
	handler := RateLimit(logger, limiter, false, false, next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(identity.WithContext(req.Context(), identity.Identity{UserID: "user-42"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if limiter.key != "user:user-42" {
		t.Fatalf("unexpected key %q", limiter.key)
	}
	if rec.Header().Get("Retry-After") != "2" {
		t.Fatalf("expected Retry-After=2, got %q", rec.Header().Get("Retry-After"))
	}
}

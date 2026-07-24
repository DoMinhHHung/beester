package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestRequestIDGeneratesUUIDv7(t *testing.T) {
	var capturedRequestID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedRequestID == "" {
		t.Fatal("expected request ID in context")
	}

	id, err := uuid.Parse(capturedRequestID)
	if err != nil {
		t.Fatalf("parse request ID: %v", err)
	}

	if id.Version() != 7 {
		t.Fatalf(
			"expected UUID version 7, got %d",
			id.Version(),
		)
	}

	if got := rec.Header().Get(RequestIDHeader); got != capturedRequestID {
		t.Fatalf(
			"expected response request ID %q, got %q",
			capturedRequestID,
			got,
		)
	}
}

func TestRequestIDPreservesValidIncomingID(t *testing.T) {
	incomingID := uuid.Must(uuid.NewV7()).String()

	var capturedRequestID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
	})

	handler := RequestID(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, incomingID)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedRequestID != incomingID {
		t.Fatalf(
			"expected request ID %q, got %q",
			incomingID,
			capturedRequestID,
		)
	}
}

func TestRequestIDReplacesInvalidIncomingID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())

		id, err := uuid.Parse(requestID)
		if err != nil {
			t.Fatalf("parse generated request ID: %v", err)
		}

		if id.Version() != 7 {
			t.Fatalf(
				"expected UUID version 7, got %d",
				id.Version(),
			)
		}
	})

	handler := RequestID(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "not-a-uuid")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

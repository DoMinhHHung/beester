package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAccessLog(t *testing.T) {
	var logs bytes.Buffer

	logger := slog.New(
		slog.NewTextHandler(&logs, nil),
	)

	requestID := uuid.Must(uuid.NewV7()).String()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)

		if _, err := w.Write([]byte("hello")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})

	handler := RequestID(
		AccessLog(logger, next),
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/users?token=secret",
		nil,
	)
	req.Header.Set(RequestIDHeader, requestID)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logs.String()

	expectedValues := []string{
		"msg=\"http request completed\"",
		"request_id=" + requestID,
		"method=POST",
		"path=/users",
		"status=201",
		"bytes_written=5",
	}

	for _, expected := range expectedValues {
		if !strings.Contains(logOutput, expected) {
			t.Fatalf(
				"expected log to contain %q, got %q",
				expected,
				logOutput,
			)
		}
	}

	if strings.Contains(logOutput, "secret") {
		t.Fatalf(
			"expected query string not to be logged, got %q",
			logOutput,
		)
	}
}

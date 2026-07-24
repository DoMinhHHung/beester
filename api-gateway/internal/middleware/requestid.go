package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestIDFromHeader(r)

		if requestID == "" {
			requestID = uuid.Must(uuid.NewV7()).String()
		}

		ctx := context.WithValue(
			r.Context(),
			requestIDContextKey{},
			requestID,
		)

		w.Header().Set(RequestIDHeader, requestID)

		next.ServeHTTP(
			w,
			r.WithContext(ctx),
		)
	})
}

func GetRequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)

	return requestID
}

func requestIDFromHeader(r *http.Request) string {
	value := r.Header.Get(RequestIDHeader)
	if value == "" {
		return ""
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return ""
	}

	return id.String()
}

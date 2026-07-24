package middleware

import (
	"net/http"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"github.com/google/uuid"
)

const RequestIDHeader = requestid.Header

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestIDFromHeader(r)

		if id == "" {
			id = uuid.Must(uuid.NewV7()).String()
		}

		ctx := requestid.WithContext(
			r.Context(),
			id,
		)

		w.Header().Set(RequestIDHeader, id)

		next.ServeHTTP(
			w,
			r.WithContext(ctx),
		)
	})
}

func requestIDFromHeader(r *http.Request) string {
	value := r.Header.Get(RequestIDHeader)
	if value == "" {
		return ""
	}

	if _, err := uuid.Parse(value); err != nil {
		return ""
	}

	return value
}

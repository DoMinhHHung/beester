package middleware

import (
	"log/slog"
	"net/http"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
)

func Recovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(
					r.Context(),
					"panic recovered",
					slog.String("request_id", requestid.FromContext(r.Context())),
					slog.Any("panic", recovered),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/middleware"
)

const readinessTimeout = 2 * time.Second

func NewHandler(
	logger *slog.Logger,
	readinessCheck func(context.Context) error,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte("ok\n")); err != nil {
			logger.Error(
				"write health response",
				slog.Any("error", err),
			)
		}
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(
			r.Context(),
			readinessTimeout,
		)
		defer cancel()

		if readinessCheck != nil {
			if err := readinessCheck(ctx); err != nil {
				http.Error(
					w,
					"not ready",
					http.StatusServiceUnavailable,
				)

				return
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte("ready\n")); err != nil {
			logger.Error(
				"write readiness response",
				slog.Any("error", err),
			)
		}
	})

	return middleware.RequestID(
		middleware.AccessLog(
			logger,
			mux,
		),
	)
}

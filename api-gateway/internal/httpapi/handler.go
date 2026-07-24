package httpapi

import (
	"github.com/DoMinhHHung/beester/api-gateway/internal/middleware"
	"log/slog"
	"net/http"
)

func NewHandler(logger *slog.Logger) http.Handler {
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

	return middleware.RequestID(
		middleware.AccessLog(
			logger,
			mux,
		),
	)
}

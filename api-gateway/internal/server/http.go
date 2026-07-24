package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

func NewHTTPServer(
	addr string,
	logger *slog.Logger,
) *HTTPServer {
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

	return &HTTPServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		logger: logger,
	}
}

func (s *HTTPServer) Run() error {
	s.logger.Info(
		"http server listening",
		slog.String("addr", s.server.Addr),
	)

	err := s.server.ListenAndServe()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return fmt.Errorf("listen and serve: %w", err)
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	return nil
}

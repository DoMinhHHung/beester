package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/config"
	"github.com/DoMinhHHung/beester/api-gateway/internal/httpapi"
	"github.com/DoMinhHHung/beester/api-gateway/internal/server"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)

	if err := run(logger); err != nil {
		logger.Error(
			"application stopped",
			slog.Any("error", err),
		)

		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger.Info(
		"application starting",
		slog.String("app_env", cfg.AppEnv),
		slog.String("http_addr", cfg.HTTPAddr),
	)

	handler := httpapi.NewHandler(
		logger,
		func(context.Context) error {
			return nil
		},
	)

	httpServer := server.NewHTTPServer(
		cfg.HTTPAddr,
		handler,
		logger,
	)

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- httpServer.Run()
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("run http server: %w", err)
		}

		return nil

	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	if err := <-serverErr; err != nil {
		return fmt.Errorf("wait for http server: %w", err)
	}

	logger.Info("application stopped gracefully")

	return nil
}

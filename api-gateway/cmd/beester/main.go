package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcclient"
	"github.com/DoMinhHHung/beester/api-gateway/internal/upstream"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/DoMinhHHung/beester/api-gateway/internal/config"
	"github.com/DoMinhHHung/beester/api-gateway/internal/httpapi"
	"github.com/DoMinhHHung/beester/api-gateway/internal/readiness"
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
	transportCredentials, err := newGRPCTransportCredentials(
		cfg.GRPCTransportSecurity,
	)
	if err != nil {
		return fmt.Errorf(
			"create gRPC transport credentials: %w",
			err,
		)
	}

	upstreamSpecs := make(
		[]upstream.Spec,
		0,
		len(cfg.GRPCUpstreams),
	)

	for _, configuredUpstream := range cfg.GRPCUpstreams {
		upstreamSpecs = append(
			upstreamSpecs,
			upstream.Spec{
				Name:   configuredUpstream.Name,
				Target: configuredUpstream.Target,
			},
		)
	}

	upstreamRegistry, err := upstream.New(
		upstreamSpecs,
		transportCredentials,
	)
	if err != nil {
		return fmt.Errorf(
			"create upstream registry: %w",
			err,
		)
	}

	defer func() {
		if err := upstreamRegistry.Close(); err != nil {
			logger.Error(
				"close upstream registry",
				slog.Any("error", err),
			)
		}
	}()

	logger.Info(
		"gRPC upstream registry initialized",
		slog.Int(
			"upstream_count",
			len(upstreamSpecs),
		),
	)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger.Info(
		"application starting",
		slog.String("app_env", cfg.AppEnv),
		slog.String("http_addr", cfg.HTTPAddr),
	)

	readinessChecks := make(
		[]readiness.Check,
		0,
		len(upstreamSpecs),
	)

	for _, spec := range upstreamSpecs {
		conn, ok := upstreamRegistry.Conn(spec.Name)
		if !ok {
			return fmt.Errorf(
				"lookup configured upstream %q",
				spec.Name,
			)
		}

		upstreamName := spec.Name
		upstreamConn := conn

		readinessChecks = append(
			readinessChecks,
			readiness.Check{
				Name: "grpc:" + upstreamName,
				Run: func(ctx context.Context) error {
					return grpcclient.CheckHealth(
						ctx,
						upstreamConn,
						"",
					)
				},
			},
		)
	}

	readinessChecker, err := readiness.New(
		readinessChecks...,
	)
	if err != nil {
		return fmt.Errorf(
			"create readiness checker: %w",
			err,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"create readiness checker: %w",
			err,
		)
	}

	handler := httpapi.NewHandler(
		logger,
		readinessChecker.Check,
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

func newGRPCTransportCredentials(
	mode config.GRPCTransportSecurity,
) (credentials.TransportCredentials, error) {
	switch mode {
	case config.GRPCTransportSecurityTLS:
		return credentials.NewTLS(
			&tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		), nil

	case config.GRPCTransportSecurityInsecure:
		return insecure.NewCredentials(), nil

	default:
		return nil, fmt.Errorf(
			"unsupported gRPC transport security %q",
			mode,
		)
	}
}

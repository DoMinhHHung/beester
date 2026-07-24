package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/auth"
	"github.com/DoMinhHHung/beester/api-gateway/internal/config"
	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcclient"
	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcmiddleware"
	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcproxy"
	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcrouting"
	"github.com/DoMinhHHung/beester/api-gateway/internal/httpapi"
	"github.com/DoMinhHHung/beester/api-gateway/internal/middleware"
	"github.com/DoMinhHHung/beester/api-gateway/internal/proxy"
	"github.com/DoMinhHHung/beester/api-gateway/internal/ratelimit"
	"github.com/DoMinhHHung/beester/api-gateway/internal/readiness"
	"github.com/DoMinhHHung/beester/api-gateway/internal/routing"
	"github.com/DoMinhHHung/beester/api-gateway/internal/server"
	"github.com/DoMinhHHung/beester/api-gateway/internal/upstream"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const shutdownTimeout = 10 * time.Second

type serverResult struct {
	name string
	err  error
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped", slog.Any("error", err))
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
		slog.String("grpc_addr", cfg.GRPCAddr),
	)

	transportCredentials, err := newGRPCTransportCredentials(cfg.GRPCTransportSecurity)
	if err != nil {
		return fmt.Errorf("create gRPC transport credentials: %w", err)
	}

	grpcSpecs := make([]upstream.Spec, 0, len(cfg.GRPCUpstreams))
	for _, configured := range cfg.GRPCUpstreams {
		grpcSpecs = append(grpcSpecs, upstream.Spec{Name: configured.Name, Target: configured.Target})
	}
	grpcRegistry, err := upstream.New(grpcSpecs, transportCredentials)
	if err != nil {
		return fmt.Errorf("create gRPC upstream registry: %w", err)
	}
	defer func() {
		if err := grpcRegistry.Close(); err != nil {
			logger.Error("close gRPC upstream registry", slog.Any("error", err))
		}
	}()

	httpSpecs := make([]upstream.HTTPSpec, 0, len(cfg.HTTPUpstreams))
	for _, configured := range cfg.HTTPUpstreams {
		httpSpecs = append(httpSpecs, upstream.HTTPSpec{Name: configured.Name, Target: configured.Target})
	}
	forwardAuthorization := !cfg.JWT.Enabled || cfg.JWT.ForwardAuthorization
	httpRegistry, err := upstream.NewHTTP(httpSpecs, logger, cfg.JWT.UserIDHeader, forwardAuthorization)
	if err != nil {
		return fmt.Errorf("create HTTP upstream registry: %w", err)
	}
	defer httpRegistry.CloseIdleConnections()

	httpRoutes := make([]routing.Route, 0, len(cfg.HTTPRoutes))
	for _, configured := range cfg.HTTPRoutes {
		httpRoutes = append(httpRoutes, routing.Route{
			Method: configured.Method, Pattern: configured.Pattern, Upstream: configured.Upstream,
		})
	}
	httpRouteTable, err := routing.New(httpRoutes)
	if err != nil {
		return fmt.Errorf("create HTTP route table: %w", err)
	}

	grpcRoutes := make([]grpcrouting.Route, 0, len(cfg.GRPCRoutes))
	for _, configured := range cfg.GRPCRoutes {
		grpcRoutes = append(grpcRoutes, grpcrouting.Route{Prefix: configured.Prefix, Upstream: configured.Upstream})
	}
	grpcRouteTable, err := grpcrouting.New(grpcRoutes)
	if err != nil {
		return fmt.Errorf("create gRPC route table: %w", err)
	}

	logger.Info(
		"gateway routes initialized",
		slog.Int("http_routes", httpRouteTable.Len()),
		slog.Int("grpc_routes", grpcRouteTable.Len()),
		slog.Int("http_upstreams", len(httpRegistry.Names())),
		slog.Int("grpc_upstreams", len(grpcRegistry.Names())),
	)

	jwtValidator, err := newJWTValidator(cfg)
	if err != nil {
		return err
	}

	redisClient, limiter, err := newRateLimiter(cfg)
	if err != nil {
		return err
	}
	if redisClient != nil {
		defer func() {
			if err := redisClient.Close(); err != nil {
				logger.Error("close Redis client", slog.Any("error", err))
			}
		}()
	}

	readinessChecker, err := newReadinessChecker(cfg, grpcRegistry, redisClient)
	if err != nil {
		return fmt.Errorf("create readiness checker: %w", err)
	}

	dispatcher := proxy.NewHTTPDispatcher(httpRouteTable, httpRegistry, logger, cfg.UpstreamTimeout)
	var gatewayHandler http.Handler = dispatcher
	gatewayHandler = middleware.BodyLimit(cfg.MaxRequestBodyBytes, gatewayHandler)
	if limiter != nil {
		gatewayHandler = middleware.RateLimit(logger, limiter, cfg.RateLimit.FailOpen, cfg.RateLimit.TrustProxy, gatewayHandler)
	}
	if jwtValidator != nil {
		gatewayHandler = middleware.JWTAuth(jwtValidator, cfg.JWT.PublicPathPrefixes, cfg.JWT.UserIDHeader, gatewayHandler)
	}

	httpHandler := httpapi.NewHandler(logger, readinessChecker.Check, httpapi.WithGatewayHandler(gatewayHandler))
	httpServer := server.NewHTTPServer(cfg.HTTPAddr, httpHandler, logger)

	var grpcServer *server.GRPCServer
	if cfg.GRPCAddr != "" {
		grpcProxy := grpcproxy.New(grpcRouteTable, grpcRegistry, cfg.JWT.UserIDHeader, forwardAuthorization)
		streamInterceptors := []grpc.StreamServerInterceptor{
			grpcmiddleware.RequestIDStreamInterceptor,
			grpcmiddleware.AccessLogStreamInterceptor(logger),
		}
		if jwtValidator != nil {
			streamInterceptors = append(streamInterceptors, grpcmiddleware.JWTAuthStreamInterceptor(jwtValidator, cfg.JWT.PublicGRPCMethodPrefixes))
		}
		if limiter != nil {
			streamInterceptors = append(streamInterceptors, grpcmiddleware.RateLimitStreamInterceptor(logger, limiter, cfg.RateLimit.FailOpen))
		}
		grpcServer = server.NewGRPCServer(
			cfg.GRPCAddr,
			logger,
			grpc.UnknownServiceHandler(grpcProxy.Handle),
			grpc.ForceServerCodec(grpcproxy.Codec()),
			grpc.ChainStreamInterceptor(streamInterceptors...),
		)
	}

	return runServers(logger, httpServer, grpcServer)
}

func newJWTValidator(cfg config.Config) (*auth.Validator, error) {
	if !cfg.JWT.Enabled {
		return nil, nil
	}
	validator, err := auth.NewHMACValidator(cfg.JWT.HMACSecret, cfg.JWT.Issuer, cfg.JWT.Audience, cfg.JWT.UserIDClaim, cfg.JWT.Leeway)
	if err != nil {
		return nil, fmt.Errorf("create JWT validator: %w", err)
	}
	return validator, nil
}

func newRateLimiter(cfg config.Config) (*redis.Client, ratelimit.Limiter, error) {
	if !cfg.RateLimit.Enabled {
		return nil, nil, nil
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.RateLimit.RedisAddr, Password: cfg.RateLimit.RedisPassword, DB: cfg.RateLimit.RedisDB})
	limiter, err := ratelimit.NewRedisLimiter(client, cfg.RateLimit.Capacity, cfg.RateLimit.RefillPerSecond, cfg.RateLimit.KeyPrefix)
	if err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("create rate limiter: %w", err)
	}
	return client, limiter, nil
}

func newReadinessChecker(cfg config.Config, grpcRegistry *upstream.Registry, redisClient *redis.Client) (*readiness.Checker, error) {
	checks := make([]readiness.Check, 0, len(cfg.GRPCUpstreams)+1)
	for _, spec := range cfg.GRPCUpstreams {
		conn, ok := grpcRegistry.Conn(spec.Name)
		if !ok {
			return nil, fmt.Errorf("lookup configured gRPC upstream %q", spec.Name)
		}
		name, upstreamConn := spec.Name, conn
		checks = append(checks, readiness.Check{Name: "grpc:" + name, Run: func(ctx context.Context) error {
			return grpcclient.CheckHealth(ctx, upstreamConn, "")
		}})
	}
	if redisClient != nil {
		checks = append(checks, readiness.Check{Name: "redis", Run: func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }})
	}
	return readiness.New(checks...)
}

func runServers(logger *slog.Logger, httpServer *server.HTTPServer, grpcServer *server.GRPCServer) error {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverCount := 1
	results := make(chan serverResult, 2)
	go func() { results <- serverResult{name: "http", err: httpServer.Run()} }()
	if grpcServer != nil {
		serverCount++
		go func() { results <- serverResult{name: "grpc", err: grpcServer.Run()} }()
	}

	received := 0
	var runErrors []error
	select {
	case result := <-results:
		received++
		if result.err != nil {
			runErrors = append(runErrors, fmt.Errorf("run %s server: %w", result.name, result.err))
		} else {
			runErrors = append(runErrors, fmt.Errorf("%s server stopped unexpectedly", result.name))
		}
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	var shutdownErrors []error
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		shutdownErrors = append(shutdownErrors, err)
	}
	if grpcServer != nil {
		if err := grpcServer.Shutdown(shutdownCtx); err != nil {
			shutdownErrors = append(shutdownErrors, err)
		}
	}

	for received < serverCount {
		select {
		case result := <-results:
			received++
			if result.err != nil {
				runErrors = append(runErrors, fmt.Errorf("wait for %s server: %w", result.name, result.err))
			}
		case <-shutdownCtx.Done():
			shutdownErrors = append(shutdownErrors, fmt.Errorf("wait for servers: %w", shutdownCtx.Err()))
			received = serverCount
		}
	}

	if err := errors.Join(append(runErrors, shutdownErrors...)...); err != nil {
		return err
	}
	logger.Info("application stopped gracefully")
	return nil
}

func newGRPCTransportCredentials(mode config.GRPCTransportSecurity) (credentials.TransportCredentials, error) {
	switch mode {
	case config.GRPCTransportSecurityTLS:
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}), nil
	case config.GRPCTransportSecurityInsecure:
		return insecure.NewCredentials(), nil
	default:
		return nil, fmt.Errorf("unsupported gRPC transport security %q", mode)
	}
}

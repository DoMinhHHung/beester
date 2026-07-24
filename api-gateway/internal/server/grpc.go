package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
)

type GRPCServer struct {
	addr   string
	server *grpc.Server
	logger *slog.Logger
}

func NewGRPCServer(addr string, logger *slog.Logger, options ...grpc.ServerOption) *GRPCServer {
	return &GRPCServer{
		addr:   addr,
		server: grpc.NewServer(options...),
		logger: logger,
	}
}

func (s *GRPCServer) Run() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}

	s.logger.Info("gRPC server listening", slog.String("addr", s.addr))
	if err := s.server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("serve gRPC: %w", err)
	}
	return nil
}

func (s *GRPCServer) Shutdown(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}

	s.logger.Info("gRPC server shutting down")
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.server.Stop()
		<-done
		return fmt.Errorf("graceful stop gRPC server: %w", ctx.Err())
	}
}

package grpcproxy

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcrouting"
	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"github.com/DoMinhHHung/beester/api-gateway/internal/upstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type frame struct {
	payload []byte
}

type rawCodec struct{}

func (rawCodec) Name() string { return "proto" }

func (rawCodec) Marshal(value any) ([]byte, error) {
	message, ok := value.(*frame)
	if !ok {
		return nil, fmt.Errorf("raw gRPC codec: expected *frame, got %T", value)
	}
	return message.payload, nil
}

func (rawCodec) Unmarshal(data []byte, value any) error {
	message, ok := value.(*frame)
	if !ok {
		return fmt.Errorf("raw gRPC codec: expected *frame, got %T", value)
	}
	message.payload = append(message.payload[:0], data...)
	return nil
}

func Codec() encoding.Codec { return rawCodec{} }

type Proxy struct {
	routes               *grpcrouting.Table
	upstreams            *upstream.Registry
	userIDMetadataKey    string
	forwardAuthorization bool
}

func New(
	routes *grpcrouting.Table,
	upstreams *upstream.Registry,
	userIDMetadataKey string,
	forwardAuthorization bool,
) *Proxy {
	return &Proxy{
		routes:               routes,
		upstreams:            upstreams,
		userIDMetadataKey:    strings.ToLower(strings.TrimSpace(userIDMetadataKey)),
		forwardAuthorization: forwardAuthorization,
	}
}

func (p *Proxy) Handle(_ any, serverStream grpc.ServerStream) error {
	if p == nil || p.routes == nil || p.upstreams == nil {
		return status.Error(codes.Unavailable, "gRPC gateway is not initialized")
	}

	method, ok := grpc.MethodFromServerStream(serverStream)
	if !ok || method == "" {
		return status.Error(codes.Internal, "gRPC method is unavailable")
	}

	route, ok := p.routes.Match(method)
	if !ok {
		return status.Errorf(codes.Unimplemented, "no gRPC route for %s", method)
	}

	conn, ok := p.upstreams.Conn(route.Upstream)
	if !ok {
		return status.Errorf(codes.Unavailable, "gRPC upstream %q is unavailable", route.Upstream)
	}

	ctx, cancel := context.WithCancel(serverStream.Context())
	defer cancel()

	outgoingMD := proxyMetadata(ctx, p.userIDMetadataKey, p.forwardAuthorization)
	outgoingCtx := metadata.NewOutgoingContext(ctx, outgoingMD)

	clientStream, err := conn.NewStream(
		outgoingCtx,
		&grpc.StreamDesc{ServerStreams: true, ClientStreams: true},
		method,
		grpc.ForceCodec(rawCodec{}),
	)
	if err != nil {
		return err
	}

	clientToBackend := make(chan error, 1)
	backendToClient := make(chan error, 1)
	go func() { clientToBackend <- forwardClientToBackend(serverStream, clientStream) }()
	go func() { backendToClient <- forwardBackendToClient(clientStream, serverStream) }()

	for clientToBackend != nil || backendToClient != nil {
		select {
		case err := <-clientToBackend:
			clientToBackend = nil
			if err == nil || err == io.EOF {
				if closeErr := clientStream.CloseSend(); closeErr != nil {
					return closeErr
				}
				continue
			}
			return err

		case err := <-backendToClient:
			backendToClient = nil
			if err == nil || err == io.EOF {
				return nil
			}
			return err
		}
	}

	return nil
}

func forwardClientToBackend(serverStream grpc.ServerStream, clientStream grpc.ClientStream) error {
	for {
		message := &frame{}
		if err := serverStream.RecvMsg(message); err != nil {
			return err
		}
		if err := clientStream.SendMsg(message); err != nil {
			return err
		}
	}
}

func forwardBackendToClient(clientStream grpc.ClientStream, serverStream grpc.ServerStream) error {
	headers, err := clientStream.Header()
	if err != nil {
		return err
	}
	headers = headers.Copy()
	headers.Delete(requestid.MetadataKey)
	if len(headers) > 0 {
		if err := serverStream.SendHeader(headers); err != nil {
			return err
		}
	}

	for {
		message := &frame{}
		err := clientStream.RecvMsg(message)
		if err != nil {
			trailers := clientStream.Trailer().Copy()
			trailers.Delete(requestid.MetadataKey)
			serverStream.SetTrailer(trailers)
			return err
		}
		if err := serverStream.SendMsg(message); err != nil {
			return err
		}
	}
}

func proxyMetadata(ctx context.Context, userIDKey string, forwardAuthorization bool) metadata.MD {
	out := metadata.MD{}
	if incoming, ok := metadata.FromIncomingContext(ctx); ok {
		for key, values := range incoming {
			lowerKey := strings.ToLower(key)
			if strings.HasPrefix(lowerKey, "grpc-") || lowerKey == "content-type" || lowerKey == "user-agent" || lowerKey == "te" {
				continue
			}
			if !forwardAuthorization && lowerKey == "authorization" {
				continue
			}
			out[lowerKey] = append([]string(nil), values...)
		}
	}

	out.Delete(requestid.MetadataKey)
	if id := requestid.FromContext(ctx); id != "" {
		out.Set(requestid.MetadataKey, id)
	}

	if userIDKey != "" {
		out.Delete(userIDKey)
		if value, ok := identity.FromContext(ctx); ok && value.UserID != "" {
			out.Set(userIDKey, value.UserID)
		}
	}

	return out
}

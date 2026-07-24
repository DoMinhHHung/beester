package grpcclient

import (
	"context"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func UnaryRequestIDInterceptor(
	ctx context.Context,
	method string,
	req any,
	reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return invoker(
		withRequestIDMetadata(ctx),
		method,
		req,
		reply,
		cc,
		opts...,
	)
}

func StreamRequestIDInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return streamer(
		withRequestIDMetadata(ctx),
		desc,
		cc,
		method,
		opts...,
	)
}

func withRequestIDMetadata(ctx context.Context) context.Context {
	id := requestid.FromContext(ctx)
	if id == "" {
		return ctx
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}

	md.Set(requestid.MetadataKey, id)

	return metadata.NewOutgoingContext(ctx, md)
}

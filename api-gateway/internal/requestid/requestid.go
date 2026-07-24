package requestid

import "context"

const (
	Header      = "X-Request-ID"
	MetadataKey = "x-request-id"
)

type contextKey struct{}

func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)

	return id
}

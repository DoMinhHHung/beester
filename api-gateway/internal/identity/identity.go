package identity

import "context"

type Identity struct {
	UserID string
}

type contextKey struct{}

func WithContext(ctx context.Context, value Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, value)
}

func FromContext(ctx context.Context) (Identity, bool) {
	value, ok := ctx.Value(contextKey{}).(Identity)
	return value, ok
}

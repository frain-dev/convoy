package drain

import "context"

type actorKey struct{}

// WithActor attaches an authenticated command origin to the durable audit trail.
func WithActor(ctx context.Context, identity string) context.Context {
	return context.WithValue(ctx, actorKey{}, identity)
}
func actor(ctx context.Context) string {
	if value, ok := ctx.Value(actorKey{}).(string); ok && value != "" {
		return value
	}
	return "runtime"
}

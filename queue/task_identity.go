package queue

import "context"

type taskIdentityKey struct{}
type taskIdentity struct{ id string }

// WithTaskIdentity is set by the broker adapter after a claim, never from
// message payload or caller-controlled headers.
func WithTaskIdentity(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, taskIdentityKey{}, taskIdentity{id: id})
}
func TaskIdentity(ctx context.Context) string {
	if v, ok := ctx.Value(taskIdentityKey{}).(taskIdentity); ok {
		return v.id
	}
	return ""
}

type authoritativeReadKey struct{}

// WithAuthoritativeReads requires handlers to read durable status under the shared execution lock.
func WithAuthoritativeReads(ctx context.Context) context.Context {
	return context.WithValue(ctx, authoritativeReadKey{}, true)
}
func AuthoritativeReads(ctx context.Context) bool {
	value, _ := ctx.Value(authoritativeReadKey{}).(bool)
	return value
}

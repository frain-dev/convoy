package task

import (
	"context"
	"time"
)

// JobLocker serializes singleton cron jobs. Construct once at boot.
//
// maxRuntime is how long the critical section may run, not a lease TTL: fn's
// context is cancelled once it elapses, the lock stays held until fn returns,
// and WithLock then reports the overrun. Callers must pass a ceiling above the
// job's realistic runtime and use the supplied context for their own work.
type JobLocker interface {
	WithLock(ctx context.Context, name string, maxRuntime time.Duration, fn func(context.Context) error) error
}

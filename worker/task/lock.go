package task

import (
	"context"
	"errors"
	"time"
)

// ErrLockBusy means the lock was not attempted or not taken because someone else
// holds it, or because the provider had no capacity to try. It is the routine
// outcome of contention, not a failure: callers skip this round and rely on
// their next tick or request. Providers must return it wrapped so callers can
// tell contention apart from a lock backend that is actually broken.
var ErrLockBusy = errors.New("lock is busy")

// JobLocker serializes singleton cron jobs. Construct once at boot.
//
// maxRuntime is how long the critical section may run, not a lease TTL: fn's
// context is cancelled once it elapses, the lock stays held until fn returns,
// and WithLock then reports the overrun. Callers must pass a ceiling above the
// job's realistic runtime and use the supplied context for their own work.
type JobLocker interface {
	WithLock(ctx context.Context, name string, maxRuntime time.Duration, fn func(context.Context) error) error
}

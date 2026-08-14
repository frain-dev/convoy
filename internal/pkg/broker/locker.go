package broker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/internal/pkg/pglock"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/worker/task"
)

// ErrLockLeaseLost means a Redis lease renew failed (transport error or
// definitive lost-lock). Both are treated as lost: fail closed for singleton jobs.
var ErrLockLeaseLost = errors.New("lock lease could not be renewed")

type postgresJobLocker struct {
	db     *sqlx.DB
	logger log.Logger
	slots  chan struct{}
}

// runBounded gives both providers one meaning for maxRuntime: fn runs with a
// context bounded by it, the caller keeps holding the lock until fn returns,
// and an overrun is reported once fn is done.
func runBounded(ctx context.Context, maxRuntime time.Duration, fn func(context.Context) error) error {
	runCtx := ctx
	if maxRuntime > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, maxRuntime)
		defer cancel()
	}

	if err := fn(runCtx); err != nil {
		return err
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("lock section exceeded max runtime: %w", runCtx.Err())
	}
	return nil
}

func (l *postgresJobLocker) WithLock(ctx context.Context, name string, maxRuntime time.Duration, fn func(context.Context) error) error {
	// Slot gate matches the dedicated pool MaxOpenConns so waiters fail with
	// ctx errors instead of blocking forever behind a saturated lock pool.
	select {
	case l.slots <- struct{}{}:
		defer func() { <-l.slots }()
	case <-ctx.Done():
		return fmt.Errorf("failed to obtain lock: %v", ctx.Err())
	}

	mu, err := pglock.TryLock(ctx, l.db, name)
	if err != nil {
		return fmt.Errorf("failed to obtain lock: %v", err)
	}
	// Failure policy: the session advisory lock is held until fn returns, on
	// success and on error. maxRuntime cancels fn's context instead of releasing
	// the lock, so a wedged job is bounded without opening a second-run window.
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if unlockErr := mu.Unlock(unlockCtx); unlockErr != nil {
			l.logger.Error("failed to release lock", "error", unlockErr)
		}
	}()

	return runBounded(ctx, maxRuntime, fn)
}

type redisJobLocker struct {
	client redis.UniversalClient
	logger log.Logger
}

func newRedisJobLocker(client redis.UniversalClient, logger log.Logger) task.JobLocker {
	return &redisJobLocker{client: client, logger: logger}
}

func (l *redisJobLocker) WithLock(ctx context.Context, name string, maxRuntime time.Duration, fn func(context.Context) error) error {
	mutex := redsync.New(goredis.NewPool(l.client)).NewMutex(name, redsync.WithExpiry(maxRuntime), redsync.WithTries(1))
	lockTimeout := 2 * time.Second
	if maxRuntime >= time.Minute {
		lockTimeout = 30 * time.Second
	}
	lockCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()
	if err := mutex.LockContext(lockCtx); err != nil {
		return fmt.Errorf("failed to obtain lock: %v", err)
	}

	defer func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer unlockCancel()
		ok, err := mutex.UnlockContext(unlockCtx)
		if !ok || err != nil {
			l.logger.Error("failed to release lock", "error", err)
		}
	}()
	// The lease is renewed while fn runs, so the lock outlives maxRuntime and is
	// only released after fn returns, matching the Postgres provider.
	if maxRuntime >= time.Minute {
		return runWithLeaseRenewal(ctx, maxRuntime, maxRuntime/3, mutex.ExtendContext, l.logger, fn)
	}
	return runBounded(ctx, maxRuntime, fn)
}

// runWithLeaseRenewal periodically extends a lease while fn runs.
// Failure policy: any extend failure (transport error or definitive lost-lock /
// !ok) is treated as lost and fail-closed for singleton jobs: cancel fn's
// context, keep holding until fn returns, and return ErrLockLeaseLost so a
// truncated run is not mistaken for success.
func runWithLeaseRenewal(
	ctx context.Context,
	maxRuntime time.Duration,
	renewEvery time.Duration,
	extend func(context.Context) (bool, error),
	logger log.Logger,
	fn func(context.Context) error,
) error {
	runParent, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	var (
		leaseMu  sync.Mutex
		leaseErr error
	)
	renewDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(renewEvery)
		defer ticker.Stop()
		for {
			select {
			case <-renewDone:
				return
			case <-ticker.C:
				renewCtx, renewCancel := context.WithTimeout(ctx, 10*time.Second)
				ok, err := extend(renewCtx)
				renewCancel()
				if ok && err == nil {
					continue
				}
				logger.Error("failed to renew lock lease", "error", err)
				leaseMu.Lock()
				if leaseErr == nil {
					if err != nil {
						leaseErr = fmt.Errorf("%w: %v", ErrLockLeaseLost, err)
					} else {
						leaseErr = ErrLockLeaseLost
					}
				}
				leaseMu.Unlock()
				cancelRun()
				return
			}
		}
	}()
	defer close(renewDone)

	runErr := runBounded(runParent, maxRuntime, fn)

	leaseMu.Lock()
	le := leaseErr
	leaseMu.Unlock()
	if le != nil {
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			return fmt.Errorf("%w (callback: %v)", le, runErr)
		}
		return le
	}
	return runErr
}

var _ task.JobLocker = (*postgresJobLocker)(nil)
var _ task.JobLocker = (*redisJobLocker)(nil)

package task

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"

	log "github.com/frain-dev/convoy/pkg/logger"
)

// redisJobLocker preserves the existing Redis-backed integration-test fixture.
// Production adapter construction lives in the broker composition package.
type redisJobLocker struct {
	rd     redis.UniversalClient
	logger log.Logger
}

func (l *redisJobLocker) WithLock(ctx context.Context, name string, maxRuntime time.Duration, fn func(context.Context) error) error {
	mutex := redsync.New(goredis.NewPool(l.rd)).NewMutex(name, redsync.WithExpiry(maxRuntime), redsync.WithTries(1))
	if err := mutex.LockContext(ctx); err != nil {
		return fmt.Errorf("failed to obtain lock: %v", err)
	}
	defer func() {
		_, _ = mutex.UnlockContext(ctx)
	}()
	return fn(ctx)
}

//go:build integration

package drain

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func redisDockerFence(t *testing.T) RedisFence {
	t.Helper()
	r := dockerRepository(t)
	admin := redis.NewClient(&redis.Options{Addr: "redis:6379", Username: "queue_controller", Password: "disposable-controller", ContextTimeoutEnabled: true, MaxRetries: -1})
	t.Cleanup(func() { require.NoError(t, admin.Close()) })
	return RedisFence{Repository: r, Admin: admin, Scope: "redis-fence", StoreID: "redis-store", ProducerUser: "default", ExecutorUser: "queue_executor", ControllerUser: "queue_controller"}
}

func TestDockerRedisStoreFence(t *testing.T) {
	// All credentials in this test belong only to the disposable Compose
	// instance. The runner never reads host queue credentials or configuration.
	f := redisDockerFence(t)
	legacy := redis.NewClient(&redis.Options{Addr: "redis:6379", ContextTimeoutEnabled: true, MaxRetries: -1})
	defer legacy.Close()
	require.NoError(t, legacy.ACLSetUser(t.Context(), "queue_controller", "reset", "on", ">disposable-controller", "~*", "&*", "+@all").Err())
	require.NoError(t, legacy.ACLSetUser(t.Context(), "queue_executor", "reset", "on", ">disposable-executor", "~*", "&*", "+@all", "-@admin").Err())
	require.NoError(t, f.Bind(t.Context()))
	require.NoError(t, f.Bind(t.Context()))
	foreign := f
	foreign.Scope = "foreign-deployment"
	require.ErrorIs(t, foreign.Bind(t.Context()), ErrConflict)
	require.NoError(t, f.Admin.ACLSetUser(t.Context(), "unregistered", "reset", "off").Err())
	require.Error(t, f.Check(t.Context()), "unknown users must not be silently disabled")
	require.NoError(t, f.Admin.ACLDelUser(t.Context(), "unregistered").Err())
	require.NoError(t, legacy.Set(t.Context(), "legacy-before-fence", "retained", 0).Err())
	target := Target{Scope: f.Scope, StoreID: f.StoreID, Provider: "redis", ConfigurationRevision: "redis-cfg", Purpose: Quiesce}
	op, err := f.Repository.Begin(t.Context(), target, "redis-fence-begin")
	require.NoError(t, err)
	require.NoError(t, f.Close(t.Context(), op))
	require.NoError(t, f.Close(t.Context(), op))
	closed, err := f.Closed(t.Context(), op)
	require.NoError(t, err)
	require.True(t, closed)
	require.Error(t, legacy.Set(t.Context(), "legacy-after-fence", "must-not-write", 0).Err(), "an already authenticated connection must lose authority")
	executor := redis.NewClient(&redis.Options{Addr: "redis:6379", Username: "queue_executor", Password: "disposable-executor", ContextTimeoutEnabled: true, MaxRetries: -1})
	defer executor.Close()
	require.NoError(t, executor.Set(t.Context(), "executor-after-fence", "retained", 0).Err())
	require.Error(t, executor.ACLSetUser(t.Context(), "default", "on").Err())
	value, err := executor.Get(t.Context(), "legacy-before-fence").Result()
	require.NoError(t, err)
	require.Equal(t, "retained", value, "fencing must not remove existing work")
	require.ErrorIs(t, f.Open(t.Context(), op), ErrConflict)
	// Leave the operation fenced. The runner now restarts Redis itself and
	// verifies ACL and ownership persistence from a separate process.
}

func TestDockerRedisFenceAfterRestart(t *testing.T) {
	f := redisDockerFence(t)
	op, err := f.Repository.Current(t.Context(), f.Scope)
	require.NoError(t, err)
	require.NotNil(t, op)
	closed, err := f.Closed(t.Context(), *op)
	require.NoError(t, err)
	require.True(t, closed)
	legacy := redis.NewClient(&redis.Options{Addr: "redis:6379", ContextTimeoutEnabled: true, MaxRetries: -1, DialTimeout: time.Second})
	defer legacy.Close()
	require.Error(t, legacy.Set(t.Context(), "restart-refill", "must-not-write", 0).Err())
	resuming, err := f.Repository.Advance(t.Context(), f.Scope, op.ID, op.Revision, Resuming, Evidence{})
	require.NoError(t, err)
	require.NoError(t, f.Open(t.Context(), resuming))
	require.NoError(t, f.Open(t.Context(), resuming))
	require.ErrorIs(t, f.Close(t.Context(), *op), ErrStale, "a stale controller must not undo resume")
	require.NoError(t, legacy.Set(t.Context(), "resumed-producer", "accepted", 0).Err())
	_, err = f.Repository.Advance(t.Context(), f.Scope, resuming.ID, resuming.Revision, Resumed, Evidence{Epoch: resuming.Epoch, ConfigurationRevision: resuming.ConfigurationRevision, RuntimeReady: true, AdmissionOpen: true})
	require.NoError(t, err)
}

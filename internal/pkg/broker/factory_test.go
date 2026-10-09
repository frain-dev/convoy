package broker

import (
	"database/sql"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license/noop"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func testConfig(provider config.QueueProvider) config.Configuration {
	cfg := config.Configuration{
		QueueProvider:       provider,
		WorkerExecutionMode: config.DefaultExecutionMode,
		Redis: config.RedisConfiguration{
			Scheme:   "redis",
			Host:     "127.0.0.1",
			Port:     6379,
			Database: "0",
		},
	}
	if provider == config.PostgresQueueProvider {
		cfg.EnableFeatureFlag = []string{string(fflag.PostgresQueue)}
	}
	return cfg
}

func assertCompleteDependencies(t *testing.T, provider config.QueueProvider, deps *Dependencies) {
	t.Helper()
	require.NotNil(t, deps.Queue)
	require.NotNil(t, deps.QueueInspector)
	if provider == config.RedisQueueProvider {
		require.NotNil(t, deps.QueueMonitor)
	} else {
		require.Nil(t, deps.QueueMonitor)
	}
	require.NotNil(t, deps.Cache)
	require.NotNil(t, deps.RateLimiter)
	require.NotNil(t, deps.CircuitBreakerStore)
	require.NotNil(t, deps.JobLocker)
	require.NotNil(t, deps.Acker)
	require.NotNil(t, deps.TrialEvents)
	require.NotNil(t, deps.ConsumerBackend)
	require.NotNil(t, deps.Scheduler)
	require.NotNil(t, deps.TaskErrors)
	require.NotNil(t, deps.ResendClaims)
	require.NotNil(t, deps.BatchTracker)
}

func TestRegistryBuildsBothProviders(t *testing.T) {
	db := sqlx.NewDb(&sql.DB{}, "postgres")
	logger := log.New("test", log.LevelError)
	stubJobLockDB(t, db)

	postgresDeps, err := New(testConfig(config.PostgresQueueProvider), db, logger)
	require.NoError(t, err)
	assertCompleteDependencies(t, config.PostgresQueueProvider, postgresDeps)

	redisDeps, err := New(testConfig(config.RedisQueueProvider), db, logger)
	require.NoError(t, err)
	assertCompleteDependencies(t, config.RedisQueueProvider, redisDeps)
}

func stubJobLockDB(t *testing.T, db *sqlx.DB) {
	t.Helper()
	original := openJobLockDB
	t.Cleanup(func() { openJobLockDB = original })
	openJobLockDB = func(config.DatabaseConfiguration) (*sqlx.DB, error) {
		return db, nil
	}
}

func TestRegistryRejectsUnknownProviderWithoutFallback(t *testing.T) {
	deps, err := New(testConfig(config.QueueProvider("unknown")), nil, log.New("test", log.LevelError))
	require.Nil(t, deps)
	require.EqualError(t, err, `unsupported broker provider "unknown"`)
}

func TestPostgresDependencyGraphDoesNotConstructRedis(t *testing.T) {
	original := newRedisClient
	t.Cleanup(func() { newRedisClient = original })

	called := false
	newRedisClient = func(config.RedisConfiguration) (*rdb.Redis, error) {
		called = true
		return nil, nil
	}

	db := sqlx.NewDb(&sql.DB{}, "postgres")
	stubJobLockDB(t, db)
	deps, err := New(testConfig(config.PostgresQueueProvider), db, log.New("test", log.LevelError))
	require.NoError(t, err)
	require.NotNil(t, deps)
	require.False(t, called)
}

func TestPostgresProviderWithoutFeatureFlagDoesNotFallbackToRedis(t *testing.T) {
	original := newRedisClient
	t.Cleanup(func() { newRedisClient = original })

	called := false
	newRedisClient = func(config.RedisConfiguration) (*rdb.Redis, error) {
		called = true
		return nil, nil
	}

	cfg := testConfig(config.PostgresQueueProvider)
	cfg.EnableFeatureFlag = nil

	deps, err := New(cfg, sqlx.NewDb(&sql.DB{}, "postgres"), log.New("test", log.LevelError))
	require.Nil(t, deps)
	require.ErrorIs(t, err, fflag.ErrPostgresQueueNotEnabled)
	require.False(t, called)
}

type denyPostgresQueueLicenser struct {
	*noop.Licenser
}

func (denyPostgresQueueLicenser) PostgresQueue() bool { return false }

func TestAllowPostgresQueue(t *testing.T) {
	redisCfg := testConfig(config.RedisQueueProvider)
	require.NoError(t, AllowPostgresQueue(redisCfg, nil))

	pgCfg := testConfig(config.PostgresQueueProvider)
	require.NoError(t, AllowPostgresQueue(pgCfg, noop.NewLicenser()))
	require.EqualError(t, AllowPostgresQueue(pgCfg, nil), "postgres queue requires the postgres_queue license entitlement")
	require.EqualError(t, AllowPostgresQueue(pgCfg, denyPostgresQueueLicenser{Licenser: noop.NewLicenser()}), "postgres queue requires the postgres_queue license entitlement")
}

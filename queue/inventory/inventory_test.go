package inventory

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/frain-dev/convoy/config"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

type readerFunc func(context.Context) (Snapshot, error)

func (f readerFunc) Inspect(ctx context.Context) (Snapshot, error) { return f(ctx) }

func TestPreviousQueueVisibilityAndFailureRecovery(t *testing.T) {
	for _, provider := range []string{"redis", "postgres"} {
		t.Run(provider, func(t *testing.T) {
			counts := Counts{}
			var failure error
			previous := readerFunc(func(context.Context) (Snapshot, error) {
				return Snapshot{ObservedAt: time.Now(), Queues: []Queue{{Name: "leftover", Counts: counts}}}, failure
			})
			active := readerFunc(func(context.Context) (Snapshot, error) { return Snapshot{ObservedAt: time.Now()}, nil })
			inv, err := New([]Registration{{ID: "current", Provider: provider, Role: "active", Reader: active}, {ID: "previous", Provider: provider, Role: "previous", Reader: previous}})
			require.NoError(t, err)
			require.False(t, inv.Inspect(t.Context())[1].Visible)
			for _, c := range []Counts{{Pending: 1}, {Processing: 1}, {Scheduled: 1}, {Retry: 1}, {Future: 1}, {Aggregating: 1}, {Archived: 1}, {Unknown: 1}} {
				counts = c
				row := inv.Inspect(t.Context())[1]
				require.True(t, row.Visible)
				require.Empty(t, row.Actions, "counts never authorize execution")
			}
			failure = errors.New("secret connection string")
			failed := inv.Inspect(t.Context())[1]
			require.True(t, failed.Visible)
			require.Nil(t, failed.Snapshot, "failed read must not report zeros")
			require.Equal(t, []string{"inspection_failed"}, failed.VisibilityReasons)
			failure = nil
			counts = Counts{Completed: 10}
			require.False(t, inv.Inspect(t.Context())[1].Visible, "completed history does not mean remaining work")
		})
	}
}

func TestInventoryRejectsDuplicateIdentity(t *testing.T) {
	reader := readerFunc(func(context.Context) (Snapshot, error) { panic("must not inspect during registration") })
	_, err := New([]Registration{{ID: "same", Provider: "redis", Role: "active", Reader: reader}, {ID: "same", Provider: "redis", Role: "previous", Reader: reader}})
	require.Error(t, err)
}

func TestConfiguredStoresRequireExplicitPreviousProvider(t *testing.T) {
	for _, active := range []config.QueueProvider{config.RedisQueueProvider, config.PostgresQueueProvider} {
		cfg := config.Configuration{QueueProvider: active, Redis: config.RedisConfiguration{Scheme: "redis", Host: "localhost", Port: 6379}}
		inv, err := Configured(cfg, nil)
		require.NoError(t, err)
		require.Len(t, inv.stores, 1, "connections alone cannot create a previous store")
		cfg.QueueStoreScope = "test-region"
		cfg.PreviousQueueProvider = config.PostgresQueueProvider
		if active == config.PostgresQueueProvider {
			cfg.PreviousQueueProvider = config.RedisQueueProvider
			cfg.Redis = config.RedisConfiguration{Scheme: "redis", Host: "localhost", Port: 6379}
		}
		inv, err = Configured(cfg, nil)
		require.NoError(t, err)
		require.Len(t, inv.stores, 2)
		require.NotEqual(t, inv.stores[0].ID, inv.stores[1].ID)
		firstID := inv.stores[0].ID
		cfg.QueueProvider, cfg.PreviousQueueProvider = cfg.PreviousQueueProvider, cfg.QueueProvider
		cfg.Redis = config.RedisConfiguration{Scheme: "redis", Host: "localhost", Port: 6379}
		reverse, err := Configured(cfg, nil)
		require.NoError(t, err)
		require.Equal(t, firstID, reverse.stores[1].ID, "logical identity survives a role change")
	}
}

type fakeRedisInspector struct {
	names []string
	info  *asynq.QueueInfo
	err   error
	reads int
}

func (f *fakeRedisInspector) Queues() ([]string, error) { return f.names, nil }
func (f *fakeRedisInspector) GetQueueInfo(string) (*asynq.QueueInfo, error) {
	f.reads++
	return f.info, f.err
}

func TestRedisIncludesAuxiliaryWorkAndPreservesPausedState(t *testing.T) {
	fake := &fakeRedisInspector{names: []string{"unconfigured-queue"}, info: &asynq.QueueInfo{Pending: 1, Active: 2, Scheduled: 3, Retry: 4, Aggregating: 5, Archived: 6, Completed: 7, Paused: true}}
	snapshot, err := (Redis{Inspector: fake}).Inspect(t.Context())
	require.NoError(t, err)
	require.Equal(t, Counts{Pending: 1, Processing: 2, Scheduled: 3, Retry: 4, Aggregating: 5, Archived: 6, Completed: 7}, snapshot.Queues[0].Counts)
	require.True(t, snapshot.Queues[0].Paused)
	fake.err = asynq.ErrQueueNotFound
	snapshot, err = (Redis{Inspector: fake}).Inspect(t.Context())
	require.Error(t, err)
	require.Zero(t, snapshot)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	before := fake.reads
	_, err = (Redis{Inspector: fake}).Inspect(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, before, fake.reads)
}

func TestPostgresInspectionUsesOnlySelectAndPropagatesFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	reader := Postgres{DB: sqlx.NewDb(db, "sqlmock")}
	mock.ExpectQuery(regexp.QuoteMeta(postgresSnapshotSQL)).WillReturnRows(sqlmock.NewRows([]string{"name", "pending", "future", "processing", "archived", "completed", "unknown", "oldest_due_age_ms", "paused"}).AddRow("event", 2, 3, 4, 5, 6, 7, 123, true))
	snapshot, err := reader.Inspect(t.Context())
	require.NoError(t, err)
	require.Equal(t, Counts{Pending: 2, Future: 3, Processing: 4, Archived: 5, Completed: 6, Unknown: 7}, snapshot.Queues[0].Counts)
	mock.ExpectQuery(regexp.QuoteMeta(postgresSnapshotSQL)).WillReturnError(errors.New("disconnected"))
	snapshot, err = reader.Inspect(t.Context())
	require.Error(t, err)
	require.Zero(t, snapshot)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfiguredIdentityChangesWithStoreButNotCredentials(t *testing.T) {
	cfg := config.Configuration{QueueProvider: config.RedisQueueProvider, QueueStoreScope: "us", Redis: config.RedisConfiguration{Scheme: "redis", Host: "localhost", Port: 6379}}
	first, err := Configured(cfg, nil)
	require.NoError(t, err)
	cfg.Redis.Password = "rotated"
	rotated, err := Configured(cfg, nil)
	require.NoError(t, err)
	require.Equal(t, first.stores[0].ID, rotated.stores[0].ID)
	cfg.Redis.Database = "1"
	replaced, err := Configured(cfg, nil)
	require.NoError(t, err)
	require.NotEqual(t, first.stores[0].ID, replaced.stores[0].ID)
}

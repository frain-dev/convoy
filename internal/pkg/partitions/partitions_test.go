package partitions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/indexes"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	_ = os.Setenv("CONVOY_JWT_SECRET", "test-access-secret")
	_ = os.Setenv("CONVOY_JWT_REFRESH_SECRET", "test-refresh-secret")

	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, context.Context) {
	t.Helper()

	if testEnv == nil {
		t.Fatal("testEnv is nil - TestMain may not have run successfully")
	}

	require.NoError(t, config.LoadConfig(""))

	failedIndexRebuilds.Range(func(k, _ any) bool {
		failedIndexRebuilds.Delete(k)
		return true
	})

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	return postgres.NewFromConnection(conn), context.Background()
}

// blockingConverter stands in for the DDL so the run's state machine can be
// observed at each step. The real conversion finishes on its own schedule, which
// would make "is it still running" a race.
type blockingConverter struct {
	release chan struct{}
	started chan struct{}
	err     error
}

func newBlockingConverter() *blockingConverter {
	return &blockingConverter{release: make(chan struct{}), started: make(chan struct{}, 1)}
}

func (c *blockingConverter) run(context.Context, Table, Operation) error {
	c.started <- struct{}{}
	<-c.release
	return c.err
}

func newService(t *testing.T, db database.Database, c converter) *Service {
	t.Helper()
	return &Service{db: db, logger: log.New("partitions-test", log.LevelError), converter: c}
}

// blockingRebuilder stands in for the index build the same way blockingConverter
// stands in for the DDL, and lets a lookup failure be asked for directly.
type blockingRebuilder struct {
	index      indexes.Dropped
	lookupErr  error
	rebuildErr error
	release    chan struct{}
	started    chan struct{}
	done       atomic.Bool
}

func newBlockingRebuilder(table, index string) *blockingRebuilder {
	return &blockingRebuilder{
		index:   indexes.Dropped{Table: table, Name: index, Definition: "CREATE INDEX " + index + " ON convoy." + table + " (id)"},
		release: make(chan struct{}),
		started: make(chan struct{}, 1),
	}
}

func (r *blockingRebuilder) dropped(_ context.Context, name string) (indexes.Dropped, error) {
	if r.lookupErr != nil {
		return indexes.Dropped{}, r.lookupErr
	}
	if r.index.Name != name {
		return indexes.Dropped{}, indexes.ErrNotDropped
	}
	return r.index, nil
}

func (r *blockingRebuilder) listDropped(ctx context.Context) ([]indexes.Dropped, error) {
	if r.done.Load() {
		return nil, nil
	}
	if r.lookupErr != nil {
		if errors.Is(r.lookupErr, indexes.ErrNotDropped) {
			return nil, nil
		}
		return nil, r.lookupErr
	}
	d, err := r.dropped(ctx, r.index.Name)
	if err != nil {
		return nil, err
	}
	return []indexes.Dropped{d}, nil
}

func (r *blockingRebuilder) rebuild(context.Context, indexes.Dropped) error {
	r.started <- struct{}{}
	<-r.release
	if r.rebuildErr == nil {
		r.done.Store(true)
	}
	return r.rebuildErr
}

type multiRebuilder struct {
	order  []string
	byName map[string]*blockingRebuilder
}

func (m *multiRebuilder) dropped(ctx context.Context, name string) (indexes.Dropped, error) {
	r, ok := m.byName[name]
	if !ok {
		return indexes.Dropped{}, indexes.ErrNotDropped
	}
	return r.dropped(ctx, name)
}

func (m *multiRebuilder) listDropped(ctx context.Context) ([]indexes.Dropped, error) {
	out := make([]indexes.Dropped, 0, len(m.order))
	for _, name := range m.order {
		r, ok := m.byName[name]
		if !ok || r.done.Load() {
			continue
		}
		d, err := m.dropped(ctx, name)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (m *multiRebuilder) rebuild(ctx context.Context, d indexes.Dropped) error {
	r, ok := m.byName[d.Name]
	if !ok {
		return fmt.Errorf("unexpected rebuild %s", d.Name)
	}
	return r.rebuild(ctx, d)
}

func newRebuildService(t *testing.T, db database.Database, r rebuilder) *Service {
	t.Helper()
	return &Service{db: db, logger: log.New("partitions-test", log.LevelError), converter: newBlockingConverter(), rebuilder: r}
}

// The run row has to name the index, because the table alone does not say which
// of its indexes ran. The table comes from the dropped-index record rather than
// from the caller, so a row cannot name a table the index is not on.
func TestStartIndexRebuildRecordsTheIndexAndItsTable(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	s := newRebuildService(t, db, r)

	run, err := s.StartIndexRebuild(ctx, "idx_event_deliveries_usage", "user-1")
	require.NoError(t, err)
	<-r.started

	require.Equal(t, OperationRebuildIndex, run.Operation)
	require.Equal(t, TableEventDeliveries, run.TableName)
	require.NotNil(t, run.IndexName)
	require.Equal(t, "idx_event_deliveries_usage", *run.IndexName)

	close(r.release)
	done := waitForStatus(t, s, ctx, run.UID, StatusCompleted)
	require.NotNil(t, done.IndexName)
	require.Equal(t, "idx_event_deliveries_usage", *done.IndexName)
}

func TestStartQueuedDroppedIndexesSkipsBlockedIndexes(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("events", "idx_events_source_id")
	now := time.Now()
	r.index.BlockedAt = &now
	r.index.BlockedReason = "could not create unique index, key is duplicated"
	s := newRebuildService(t, db, r)

	s.StartQueuedDroppedIndexes(ctx)
	select {
	case <-r.started:
		t.Fatal("boot started a blocked index rebuild")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDuplicateKeyRebuildFailureIsPersistedAsBlocked(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
        VALUES ('idx_events_source_id', 'events',
                'CREATE INDEX idx_events_source_id ON convoy.events USING btree (id)')
        ON CONFLICT (index_name) DO UPDATE
            SET table_name = EXCLUDED.table_name,
                definition = EXCLUDED.definition,
                dropped_at = NOW(),
                rebuilt_at = NULL,
                blocked_at = NULL,
                blocked_reason = NULL`)
	require.NoError(t, err)

	r := newBlockingRebuilder("events", "idx_events_source_id")
	r.rebuildErr = errors.New("could not create unique index, key is duplicated")
	close(r.release)
	s := newRebuildService(t, db, r)

	run, err := s.StartIndexRebuild(ctx, "idx_events_source_id", "user-1")
	require.NoError(t, err)
	waitForStatus(t, s, ctx, run.UID, StatusFailed)

	var blockedAt sql.NullTime
	var reason sql.NullString
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
        SELECT blocked_at, blocked_reason FROM convoy.dropped_indexes
        WHERE index_name = 'idx_events_source_id' AND rebuilt_at IS NULL`).Scan(&blockedAt, &reason))
	require.True(t, blockedAt.Valid)
	require.True(t, reason.Valid)
	require.Contains(t, reason.String, "duplicate")
}

func TestDuplicateKeyFailureFallsBackToProcessSkipWhenBlockedWriteFails(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("events", "idx_events_source_id")
	r.rebuildErr = errors.New("could not create unique index, key is duplicated")
	close(r.release)
	s := newRebuildService(t, db, r)

	err := s.RunIndexRebuild(ctx, "idx_events_source_id", "user-1")
	require.Error(t, err)
	<-r.started

	_, skipped := failedIndexRebuilds.Load("idx_events_source_id")
	require.True(t, skipped, "without a debt row MarkBlocked fails, so boot must fall back to the process skip")

	s.StartQueuedDroppedIndexes(ctx)
	select {
	case <-r.started:
		t.Fatal("boot retried an index whose blocked marker could not be persisted")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestExplicitRetryClearsBlockedBeforeRebuild(t *testing.T) {
	db, ctx := setupTestDB(t)

	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition, blocked_at, blocked_reason)
        VALUES ('idx_events_source_id', 'events',
                'CREATE INDEX idx_events_source_id ON convoy.events USING btree (id)',
                NOW(), 'could not create unique index, key is duplicated')
        ON CONFLICT (index_name) DO UPDATE
            SET blocked_at = EXCLUDED.blocked_at,
                blocked_reason = EXCLUDED.blocked_reason,
                rebuilt_at = NULL`)
	require.NoError(t, err)

	r := newBlockingRebuilder("events", "idx_events_source_id")
	s := newRebuildService(t, db, r)

	run, err := s.StartIndexRebuild(ctx, "idx_events_source_id", "user-1")
	require.NoError(t, err)
	<-r.started

	var blockedAt sql.NullTime
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
        SELECT blocked_at FROM convoy.dropped_indexes
        WHERE index_name = 'idx_events_source_id' AND rebuilt_at IS NULL`).Scan(&blockedAt))
	require.False(t, blockedAt.Valid, "explicit retry must clear blocked before the rebuild runs")

	close(r.release)
	waitForStatus(t, s, ctx, run.UID, StatusCompleted)
}

func TestExplicitRetryPreservesBlockedWhenSlotNotTaken(t *testing.T) {
	db, ctx := setupTestDB(t)

	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition, blocked_at, blocked_reason)
        VALUES ('idx_events_source_id', 'events',
                'CREATE INDEX idx_events_source_id ON convoy.events USING btree (id)',
                NOW(), 'could not create unique index, key is duplicated')
        ON CONFLICT (index_name) DO UPDATE
            SET blocked_at = EXCLUDED.blocked_at,
                blocked_reason = EXCLUDED.blocked_reason,
                rebuilt_at = NULL`)
	require.NoError(t, err)

	holding := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	holder := newRebuildService(t, db, holding)
	run, err := holder.StartIndexRebuild(ctx, "idx_event_deliveries_usage", "user-1")
	require.NoError(t, err)
	<-holding.started

	_, err = newRebuildService(t, db, newBlockingRebuilder("events", "idx_events_source_id")).
		StartIndexRebuild(ctx, "idx_events_source_id", "user-2")
	require.ErrorIs(t, err, ErrRunInProgress)

	var blockedAt sql.NullTime
	var reason sql.NullString
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
        SELECT blocked_at, blocked_reason FROM convoy.dropped_indexes
        WHERE index_name = 'idx_events_source_id' AND rebuilt_at IS NULL`).Scan(&blockedAt, &reason))
	require.True(t, blockedAt.Valid, "blocked marker must survive a retry that never took the slot")
	require.True(t, reason.Valid)

	close(holding.release)
	waitForStatus(t, holder, ctx, run.UID, StatusCompleted)
}

func TestExplicitRetryKeepsProcessSkipWhenClearBlockedFails(t *testing.T) {
	db, ctx := setupTestDB(t)

	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition, blocked_at, blocked_reason)
        VALUES ('idx_events_source_id', 'events',
                'CREATE INDEX idx_events_source_id ON convoy.events USING btree (id)',
                NOW(), 'could not create unique index, key is duplicated')
        ON CONFLICT (index_name) DO UPDATE
            SET blocked_at = EXCLUDED.blocked_at,
                blocked_reason = EXCLUDED.blocked_reason,
                rebuilt_at = NULL`)
	require.NoError(t, err)

	failedIndexRebuilds.Store("idx_events_source_id", struct{}{})

	_, err = db.GetDB().ExecContext(ctx, `
        CREATE OR REPLACE FUNCTION convoy.test_block_clear_blocked()
        RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN
            IF OLD.blocked_at IS NOT NULL AND NEW.blocked_at IS NULL THEN
                RAISE EXCEPTION 'test: clear blocked blocked';
            END IF;
            RETURN NEW;
        END $$;
        CREATE TRIGGER test_block_clear_blocked
        BEFORE UPDATE ON convoy.dropped_indexes
        FOR EACH ROW EXECUTE FUNCTION convoy.test_block_clear_blocked()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.GetDB().ExecContext(context.Background(), `
            DROP TRIGGER IF EXISTS test_block_clear_blocked ON convoy.dropped_indexes;
            DROP FUNCTION IF EXISTS convoy.test_block_clear_blocked()`)
	})

	r := newBlockingRebuilder("events", "idx_events_source_id")
	s := newRebuildService(t, db, r)

	_, err = s.StartIndexRebuild(ctx, "idx_events_source_id", "user-1")
	require.Error(t, err)

	_, skipped := failedIndexRebuilds.Load("idx_events_source_id")
	require.True(t, skipped, "process skip must survive a failed ClearBlocked after the slot was taken")

	var blockedAt sql.NullTime
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
        SELECT blocked_at FROM convoy.dropped_indexes
        WHERE index_name = 'idx_events_source_id' AND rebuilt_at IS NULL`).Scan(&blockedAt))
	require.True(t, blockedAt.Valid)

	s.StartQueuedDroppedIndexes(ctx)
	select {
	case <-r.started:
		t.Fatal("boot retried after explicit retry dropped the process skip")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestStartQueuedDroppedIndexesStartsTheFirstOwed(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	s := newRebuildService(t, db, r)

	s.StartQueuedDroppedIndexes(ctx)
	<-r.started

	runs, err := s.List(ctx, 10)
	require.NoError(t, err)
	require.NotEmpty(t, runs)
	require.Equal(t, OperationRebuildIndex, runs[0].Operation)
	require.NotNil(t, runs[0].IndexName)
	require.Equal(t, "idx_event_deliveries_usage", *runs[0].IndexName)
	require.Equal(t, bootTriggeredBy, runs[0].TriggeredBy)

	close(r.release)
	waitForStatus(t, s, ctx, runs[0].UID, StatusCompleted)
}

func TestBootRebuildStartsEveryOwedIndexWhenTheSlotFrees(t *testing.T) {
	db, ctx := setupTestDB(t)
	usage := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	deliveries := newBlockingRebuilder("event_deliveries", indexes.EventDeliveriesProjectCreated)
	observed := newBlockingRebuilder("event_deliveries", indexes.EventDeliveriesProjectEventTypeCreated)
	gin := newBlockingRebuilder("events", indexes.PayloadGIN)
	s := newRebuildService(t, db, &multiRebuilder{
		order: []string{"idx_event_deliveries_usage", indexes.EventDeliveriesProjectCreated, indexes.EventDeliveriesProjectEventTypeCreated, indexes.PayloadGIN},
		byName: map[string]*blockingRebuilder{
			"idx_event_deliveries_usage":                   usage,
			indexes.EventDeliveriesProjectCreated:          deliveries,
			indexes.EventDeliveriesProjectEventTypeCreated: observed,
			indexes.PayloadGIN:                             gin,
		},
	})

	s.StartQueuedDroppedIndexes(ctx)
	<-usage.started
	select {
	case <-deliveries.started:
		t.Fatal("list index started while the usage rebuild held the slot")
	case <-observed.started:
		t.Fatal("observed-types index started while the usage rebuild held the slot")
	case <-gin.started:
		t.Fatal("payload GIN started while the usage rebuild held the slot")
	default:
	}

	close(usage.release)
	<-deliveries.started
	select {
	case <-observed.started:
		t.Fatal("observed-types index started while the list index rebuild held the slot")
	case <-gin.started:
		t.Fatal("payload GIN started while the list index rebuild held the slot")
	default:
	}

	close(deliveries.release)
	<-observed.started
	select {
	case <-gin.started:
		t.Fatal("payload GIN started while the observed-types rebuild held the slot")
	default:
	}

	close(observed.release)
	<-gin.started
	close(gin.release)

	runs, err := s.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, runs, 4)
	names := make(map[string]string, 4)
	for _, run := range runs {
		require.NotNil(t, run.IndexName)
		waitForStatus(t, s, ctx, run.UID, StatusCompleted)
		names[*run.IndexName] = run.TriggeredBy
	}
	require.Equal(t, bootTriggeredBy, names["idx_event_deliveries_usage"])
	require.Equal(t, bootTriggeredBy, names[indexes.EventDeliveriesProjectCreated])
	require.Equal(t, bootTriggeredBy, names[indexes.EventDeliveriesProjectEventTypeCreated])
	require.Equal(t, bootTriggeredBy, names[indexes.PayloadGIN])
}

func TestUserRebuildStartsTheNextOwedIndexWhenTheSlotFrees(t *testing.T) {
	db, ctx := setupTestDB(t)
	usage := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	gin := newBlockingRebuilder("events", indexes.PayloadGIN)
	s := newRebuildService(t, db, &multiRebuilder{
		order: []string{"idx_event_deliveries_usage", indexes.PayloadGIN},
		byName: map[string]*blockingRebuilder{
			"idx_event_deliveries_usage": usage,
			indexes.PayloadGIN:           gin,
		},
	})

	run, err := s.StartIndexRebuild(ctx, "idx_event_deliveries_usage", "user-1")
	require.NoError(t, err)
	<-usage.started
	select {
	case <-gin.started:
		t.Fatal("payload GIN started while the usage rebuild held the slot")
	default:
	}

	close(usage.release)
	<-gin.started
	close(gin.release)

	waitForStatus(t, s, ctx, run.UID, StatusCompleted)
	runs, err := s.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	for _, listed := range runs {
		require.NotNil(t, listed.IndexName)
		waitForStatus(t, s, ctx, listed.UID, StatusCompleted)
	}
}

func TestFailedRebuildStartsTheNextOwedIndexOnce(t *testing.T) {
	db, ctx := setupTestDB(t)
	usage := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	usage.rebuildErr = errors.New("lock_timeout")
	gin := newBlockingRebuilder("events", indexes.PayloadGIN)
	s := newRebuildService(t, db, &multiRebuilder{
		order: []string{"idx_event_deliveries_usage", indexes.PayloadGIN},
		byName: map[string]*blockingRebuilder{
			"idx_event_deliveries_usage": usage,
			indexes.PayloadGIN:           gin,
		},
	})

	s.StartQueuedDroppedIndexes(ctx)
	<-usage.started
	close(usage.release)
	<-gin.started
	close(gin.release)

	runs, err := s.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	for _, listed := range runs {
		require.NotNil(t, listed.IndexName)
		if *listed.IndexName == "idx_event_deliveries_usage" {
			waitForStatus(t, s, ctx, listed.UID, StatusFailed)
			continue
		}
		waitForStatus(t, s, ctx, listed.UID, StatusCompleted)
	}

	select {
	case <-usage.started:
		t.Fatal("failed usage rebuild started again after the next index finished")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTwoFailedRebuildsDoNotAlternate(t *testing.T) {
	db, ctx := setupTestDB(t)
	usage := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	usage.rebuildErr = errors.New("lock_timeout")
	gin := newBlockingRebuilder("events", indexes.PayloadGIN)
	gin.rebuildErr = errors.New("lock_timeout")
	s := newRebuildService(t, db, &multiRebuilder{
		order: []string{"idx_event_deliveries_usage", indexes.PayloadGIN},
		byName: map[string]*blockingRebuilder{
			"idx_event_deliveries_usage": usage,
			indexes.PayloadGIN:           gin,
		},
	})

	s.StartQueuedDroppedIndexes(ctx)
	<-usage.started
	close(usage.release)
	<-gin.started
	close(gin.release)

	runs, err := s.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	for _, listed := range runs {
		waitForStatus(t, s, ctx, listed.UID, StatusFailed)
	}

	select {
	case <-usage.started:
		t.Fatal("failed usage rebuild started again after the other failed rebuild finished")
	case <-gin.started:
		t.Fatal("failed payload GIN started again after the other failed rebuild finished")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestConversionOnANewRunnerDoesNotRetryAFailedIndex(t *testing.T) {
	db, ctx := setupTestDB(t)
	usage := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	usage.rebuildErr = errors.New("lock_timeout")
	boot := newRebuildService(t, db, usage)

	boot.StartQueuedDroppedIndexes(ctx)
	<-usage.started
	close(usage.release)
	runs, err := boot.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	waitForStatus(t, boot, ctx, runs[0].UID, StatusFailed)

	c := newBlockingConverter()
	s := &Service{db: db, logger: log.New("partitions-test", log.LevelError), converter: c, rebuilder: usage}
	run, err := s.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)
	<-c.started
	close(c.release)
	waitForStatus(t, s, ctx, run.UID, StatusCompleted)

	select {
	case <-usage.started:
		t.Fatal("conversion on a new runner retried an index that already failed this process")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDashboardRebuildRetriesANameThatFailedOnAnotherRunner(t *testing.T) {
	db, ctx := setupTestDB(t)
	first := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	first.rebuildErr = errors.New("lock_timeout")
	close(first.release)
	boot := newRebuildService(t, db, first)
	boot.StartQueuedDroppedIndexes(ctx)
	runs, err := boot.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	waitForStatus(t, boot, ctx, runs[0].UID, StatusFailed)
	_, skipped := failedIndexRebuilds.Load("idx_event_deliveries_usage")
	require.True(t, skipped)

	retry := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	close(retry.release)
	dashboard := newRebuildService(t, db, retry)
	run, err := dashboard.StartIndexRebuild(ctx, "idx_event_deliveries_usage", "user-1")
	require.NoError(t, err)
	<-retry.started
	waitForStatus(t, dashboard, ctx, run.UID, StatusCompleted)
	_, skipped = failedIndexRebuilds.Load("idx_event_deliveries_usage")
	require.False(t, skipped, "a successful retry must not keep skipping the name")
}

func TestConversionFinishStartsOwedIndexRebuilds(t *testing.T) {
	db, ctx := setupTestDB(t)
	c := newBlockingConverter()
	r := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	s := &Service{db: db, logger: log.New("partitions-test", log.LevelError), converter: c, rebuilder: r}

	run, err := s.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)
	<-c.started

	s.StartQueuedDroppedIndexes(ctx)
	select {
	case <-r.started:
		t.Fatal("index rebuild started while a conversion held the slot")
	default:
	}

	close(c.release)
	waitForStatus(t, s, ctx, run.UID, StatusCompleted)
	<-r.started
	close(r.release)
	runs, err := s.List(ctx, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(runs), 2)
	for _, listed := range runs {
		if listed.Operation == OperationRebuildIndex {
			waitForStatus(t, s, ctx, listed.UID, StatusCompleted)
		}
	}
}

func TestStartQueuedDroppedIndexesNoopsWhenNothingOwed(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	r.lookupErr = indexes.ErrNotDropped
	s := newRebuildService(t, db, r)

	s.StartQueuedDroppedIndexes(ctx)

	select {
	case <-r.started:
		t.Fatal("rebuild started for an index that was not queued")
	default:
	}

	runs, err := s.List(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, runs)
}

func TestStartQueuedDroppedIndexesNoopsWhenTheSlotIsTaken(t *testing.T) {
	db, ctx := setupTestDB(t)
	c := newBlockingConverter()
	running := newService(t, db, c)
	run, err := running.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)
	<-c.started

	r := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	newRebuildService(t, db, r).StartQueuedDroppedIndexes(ctx)

	select {
	case <-r.started:
		t.Fatal("index rebuild started while a conversion held the slot")
	default:
	}

	close(c.release)
	waitForStatus(t, running, ctx, run.UID, StatusCompleted)
}

func TestStartQueuedDroppedIndexesFailsOpenOnLookupError(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("event_deliveries", "idx_event_deliveries_usage")
	r.lookupErr = errors.New("database unreachable")
	s := newRebuildService(t, db, r)

	s.StartQueuedDroppedIndexes(ctx)

	select {
	case <-r.started:
		t.Fatal("rebuild started after a lookup error")
	default:
	}
}

// A rebuild and a conversion are the same kind of work on the same tables, so
// they share the instance-wide slot. The index being rebuilt lives on a table a
// conversion may be rewriting, and the two would contend for locks on it.
func TestARebuildAndAConversionShareTheSingleActiveSlot(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("events", "idx_events_source_id")
	s := newRebuildService(t, db, r)

	run, err := s.StartIndexRebuild(ctx, "idx_events_source_id", "user-1")
	require.NoError(t, err)
	<-r.started

	// A conversion of an unrelated table is refused while the rebuild holds the
	// slot, which is what makes the dashboard's gate agree with the server.
	c := newBlockingConverter()
	_, err = newService(t, db, c).Start(ctx, TableDeliveryAttempts, OperationPartition, "user-2")
	require.ErrorIs(t, err, ErrRunInProgress)

	close(r.release)
	waitForStatus(t, s, ctx, run.UID, StatusCompleted)

	// And the other way round: a conversion in flight refuses a rebuild.
	conversion := newBlockingConverter()
	cs := newService(t, db, conversion)
	converting, err := cs.Start(ctx, TableDeliveryAttempts, OperationPartition, "user-2")
	require.NoError(t, err)
	<-conversion.started

	_, err = newRebuildService(t, db, newBlockingRebuilder("events", "idx_events_source_id")).
		StartIndexRebuild(ctx, "idx_events_source_id", "user-3")
	require.ErrorIs(t, err, ErrRunInProgress)

	close(conversion.release)
	waitForStatus(t, cs, ctx, converting.UID, StatusCompleted)
}

// A name that identifies no pending rebuild must be refused before the slot is
// taken, or a mistyped index would lock the instance out of real work.
func TestStartIndexRebuildLeavesTheSlotFreeWhenTheIndexIsUnknown(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("events", "idx_events_source_id")
	r.lookupErr = indexes.ErrNotDropped
	s := newRebuildService(t, db, r)

	_, err := s.StartIndexRebuild(ctx, "idx_typed_wrong", "user-1")
	require.ErrorIs(t, err, indexes.ErrNotDropped)

	runs, err := s.List(ctx, 20)
	require.NoError(t, err)
	require.Empty(t, runs, "a rejected rebuild recorded a run, which holds the instance-wide slot")

	// The slot is provably still free.
	c := newBlockingConverter()
	close(c.release)
	_, err = newService(t, db, c).Start(ctx, TableEvents, OperationPartition, "user-2")
	require.NoError(t, err)
}

// A failed rebuild has to close its row out, or the row keeps the instance
// blocked and an operator has to clear it by hand.
func TestARebuildThatFailsClosesOutWithTheReason(t *testing.T) {
	db, ctx := setupTestDB(t)
	r := newBlockingRebuilder("events", "idx_events_source_id")
	r.rebuildErr = errors.New("could not create unique index, key is duplicated")
	close(r.release)
	s := newRebuildService(t, db, r)

	run, err := s.StartIndexRebuild(ctx, "idx_events_source_id", "user-1")
	require.NoError(t, err)

	failed := waitForStatus(t, s, ctx, run.UID, StatusFailed)
	require.NotNil(t, failed.Error)
	require.Equal(t, "could not create unique index, key is duplicated", *failed.Error)
	require.NotNil(t, failed.CompletedAt)
}

// The single-active guard is a CONCURRENTLY-built unique index, so it is in the
// class a killed build leaves invalid and the repair migration then drops. While
// it is gone the insert has nothing to fail on, so a start must be refused here
// instead of running beside whatever is already going.
func TestNoRunStartsWhileTheSingleActiveGuardIsMissing(t *testing.T) {
	db, ctx := setupTestDB(t)
	dropGuard(t, db, ctx)

	_, err := newService(t, db, newBlockingConverter()).Start(ctx, TableEvents, OperationPartition, "user-1")
	require.ErrorIs(t, err, ErrGuardMissing)

	_, err = newRebuildService(t, db, newBlockingRebuilder("events", "idx_events_source_id")).
		StartIndexRebuild(ctx, "idx_events_source_id", "user-1")
	require.ErrorIs(t, err, ErrGuardMissing)
}

// Rebuilding the guard is the exception, or an instance whose guard was dropped
// could never convert or rebuild anything again: the only work that ends the
// state would be refused by the state itself.
func TestTheGuardCanBeRebuiltWithoutTheGuard(t *testing.T) {
	db, ctx := setupTestDB(t)
	dropGuard(t, db, ctx)

	r := newBlockingRebuilder("partition_runs", singleActiveGuard)
	close(r.release)
	s := newRebuildService(t, db, r)

	run, err := s.StartIndexRebuild(ctx, singleActiveGuard, "user-1")
	require.NoError(t, err)
	waitForStatus(t, s, ctx, run.UID, StatusCompleted)
}

// Two guard rebuilds started at once are the case a read-then-insert check does
// not cover: both can read no open run before either inserts. With the guard
// index gone, nothing else refuses the second, so exactly one must win here.
func TestOnlyOneOfTwoConcurrentGuardRebuildsTakesTheSlot(t *testing.T) {
	db, ctx := setupTestDB(t)
	dropGuard(t, db, ctx)

	const starts = 4
	errs := make(chan error, starts)
	begin := make(chan struct{})
	for range starts {
		go func() {
			r := newBlockingRebuilder("partition_runs", singleActiveGuard)
			<-begin
			_, err := newRebuildService(t, db, r).StartIndexRebuild(ctx, singleActiveGuard, "user-1")
			errs <- err
		}()
	}
	close(begin)

	won := 0
	for range starts {
		if err := <-errs; err == nil {
			won++
		} else {
			require.ErrorIs(t, err, ErrRunInProgress)
		}
	}
	require.Equal(t, 1, won, "more than one rebuild of the guard took the slot")

	runs, err := newService(t, db, newBlockingConverter()).List(ctx, 20)
	require.NoError(t, err)
	require.Len(t, runs, 1)
}

// The same applies to a row left running from before the guard was dropped: the
// rebuild has to refuse rather than build a unique index over two running rows
// and fail on the key instead of on the reason.
func TestAGuardRebuildIsRefusedWhileAnotherRunIsOpen(t *testing.T) {
	db, ctx := setupTestDB(t)

	c := newBlockingConverter()
	cs := newService(t, db, c)
	converting, err := cs.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)
	<-c.started

	dropGuard(t, db, ctx)

	_, err = newRebuildService(t, db, newBlockingRebuilder("partition_runs", singleActiveGuard)).
		StartIndexRebuild(ctx, singleActiveGuard, "user-2")
	require.ErrorIs(t, err, ErrRunInProgress)

	close(c.release)
	waitForStatus(t, cs, ctx, converting.UID, StatusCompleted)
}

func dropGuard(t *testing.T, db database.Database, ctx context.Context) {
	t.Helper()
	_, err := db.GetDB().ExecContext(ctx, `DROP INDEX convoy.`+singleActiveGuard)
	require.NoError(t, err)
}

func waitForStatus(t *testing.T, s *Service, ctx context.Context, id string, want Status) *Run {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		run, err := s.Get(ctx, id)
		require.NoError(t, err)
		if run.Status == want {
			return run
		}
		if time.Now().After(deadline) {
			t.Fatalf("run stayed %q, wanted %q", run.Status, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Overlapping conversions would each rewrite a table while the other held locks
// on it, so the second caller is told rather than queued behind hours of work.
func TestStartRejectsASecondRunWhileOneIsRunning(t *testing.T) {
	db, ctx := setupTestDB(t)
	c := newBlockingConverter()
	s := newService(t, db, c)

	run, err := s.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)
	require.Equal(t, StatusRunning, run.Status)
	<-c.started

	// A different table is still refused: one conversion at a time is per
	// instance, because they compete for the same disk.
	_, err = s.Start(ctx, TableEventDeliveries, OperationPartition, "user-2")
	require.ErrorIs(t, err, ErrRunInProgress)

	close(c.release)
	waitForStatus(t, s, ctx, run.UID, StatusCompleted)

	// The slot is free once the first run closes out.
	second := newBlockingConverter()
	close(second.release)
	_, err = newService(t, db, second).Start(ctx, TableEventDeliveries, OperationPartition, "user-2")
	require.NoError(t, err)
}

func TestStartRecordsFailureWithTheReason(t *testing.T) {
	db, ctx := setupTestDB(t)
	c := newBlockingConverter()
	c.err = errors.New("relation convoy.events_new already exists")
	close(c.release)

	s := newService(t, db, c)

	run, err := s.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)

	failed := waitForStatus(t, s, ctx, run.UID, StatusFailed)
	require.NotNil(t, failed.Error)
	require.Equal(t, "relation convoy.events_new already exists", *failed.Error)
	require.NotNil(t, failed.CompletedAt)
}

// The CLI runs a conversion on the command's own context, so interrupting it
// cancels the conversion and the write that closes the run out. If that write
// goes with it the row stays running, and one running row blocks every later
// conversion on the instance until an operator clears it by hand.
func TestRunClosesOutEvenWhenItsContextIsCancelled(t *testing.T) {
	db, ctx := setupTestDB(t)
	c := newBlockingConverter()
	c.err = context.Canceled
	close(c.release)

	s := newService(t, db, c)

	cancelled, cancel := context.WithCancel(ctx)
	run, err := s.record(cancelled, TableEvents, OperationPartition, "cli")
	require.NoError(t, err)

	cancel()
	require.Error(t, s.convert(cancelled, run))

	after, err := s.Get(ctx, run.UID)
	require.NoError(t, err)
	require.Equal(t, StatusFailed, after.Status)
	require.NotNil(t, after.CompletedAt)
}

// The phase is what a UI shows while a conversion runs, and it comes from the
// notice stream rather than from inside the statement, which could not be read
// until the conversion committed.
func TestRecordPhaseUpdatesTheRunningRun(t *testing.T) {
	db, ctx := setupTestDB(t)
	c := newBlockingConverter()
	s := newService(t, db, c)

	run, err := s.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)
	<-c.started

	s.recordPhase(ctx, run.UID, &pgconn.Notice{Message: "creating partitions for 301 days"})
	s.recordPhase(ctx, run.UID, &pgconn.Notice{Message: "swapping in convoy.events_new"})

	progress, err := s.Get(ctx, run.UID)
	require.NoError(t, err)
	require.NotNil(t, progress.Phase)
	require.Equal(t, "swapping in convoy.events_new", *progress.Phase)
	require.Equal(t, int64(2), progress.NoticeCount)

	close(c.release)
	done := waitForStatus(t, s, ctx, run.UID, StatusCompleted)

	// A notice arriving after the run closed out must not reopen its phase: the
	// pool is shared, and unrelated statements raise notices too.
	s.recordPhase(ctx, run.UID, &pgconn.Notice{Message: "vacuuming something unrelated"})

	after, err := s.Get(ctx, run.UID)
	require.NoError(t, err)
	require.Equal(t, *done.Phase, *after.Phase)
	require.Equal(t, done.NoticeCount, after.NoticeCount)
	require.Len(t, after.Steps, len(done.Steps))
}

// The phase alone tells an operator where a conversion is, not how it got
// there, and the run outlives the process that logged the rest. Each step is
// kept with the time it arrived so a conversion that sat for an hour somewhere
// says which phase that was.
func TestRecordPhaseKeepsEveryStepInOrder(t *testing.T) {
	db, ctx := setupTestDB(t)
	c := newBlockingConverter()
	s := newService(t, db, c)

	run, err := s.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.NoError(t, err)
	<-c.started

	messages := []string{"checking the table can be converted", "building the index", "swapping it in"}
	for _, message := range messages {
		s.recordPhase(ctx, run.UID, &pgconn.Notice{Message: message})
	}

	progress, err := s.Get(ctx, run.UID)
	require.NoError(t, err)
	require.Len(t, progress.Steps, len(messages))

	for i, message := range messages {
		require.Equal(t, message, progress.Steps[i].Message)
		require.False(t, progress.Steps[i].At.IsZero(), "step %d has no time, so the list cannot show how long a phase took", i)
		if i > 0 {
			require.False(t, progress.Steps[i].At.Before(progress.Steps[i-1].At), "steps are out of order")
		}
	}

	close(c.release)
	waitForStatus(t, s, ctx, run.UID, StatusCompleted)
}

func TestListReturnsNewestFirst(t *testing.T) {
	db, ctx := setupTestDB(t)

	for _, table := range []Table{TableEvents, TableEventsSearch} {
		c := newBlockingConverter()
		close(c.release)
		s := newService(t, db, c)

		run, err := s.Start(ctx, table, OperationPartition, "user-1")
		require.NoError(t, err)
		waitForStatus(t, s, ctx, run.UID, StatusCompleted)
	}

	runs, err := newService(t, db, newBlockingConverter()).List(ctx, 0)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.Equal(t, TableEventsSearch, runs[0].TableName)
	require.Equal(t, TableEvents, runs[1].TableName)
}

// makePartitioned replaces a table with an empty partitioned parent of the same
// name. Running the real conversion to reach that shape would rewrite a table
// for minutes to prove a one-line predicate, and the cloned database is
// disposable.
func makePartitioned(t *testing.T, ctx context.Context, db database.Database, table Table) {
	t.Helper()

	_, err := db.GetDB().ExecContext(ctx, fmt.Sprintf("DROP TABLE convoy.%s CASCADE", table))
	require.NoError(t, err)

	_, err = db.GetDB().ExecContext(ctx, fmt.Sprintf(`
        CREATE TABLE convoy.%s (id CHAR(26) NOT NULL, created_at TIMESTAMPTZ NOT NULL)
        PARTITION BY RANGE (created_at)`, table))
	require.NoError(t, err)
}

func TestTableStatesReportsTheShapeOfEachTable(t *testing.T) {
	db, ctx := setupTestDB(t)
	s := newService(t, db, newBlockingConverter())

	states, err := s.TableStates(ctx)
	require.NoError(t, err)
	require.Len(t, states, len(Tables()))
	for i, state := range states {
		require.Equal(t, Tables()[i], state.Name)
		require.False(t, state.Partitioned)
		require.Equal(t, OperationPartition, state.ValidOperation())
	}

	makePartitioned(t, ctx, db, TableEventsSearch)

	states, err = s.TableStates(ctx)
	require.NoError(t, err)
	for _, state := range states {
		if state.Name == TableEventsSearch {
			require.True(t, state.Partitioned)
			require.Equal(t, OperationUnpartition, state.ValidOperation())
			continue
		}
		require.False(t, state.Partitioned)
	}
}

// Whether the parent adopted its source table or copied it decides what
// unpartitioning costs, and the page that offers the operation says so. A
// default partition alone must not be read as adoption: gopartman provisions one
// on every table it manages, and that one is an empty catch-all.
func TestTableStatesReportsWhichTablesWereAdoptedRatherThanCopied(t *testing.T) {
	db, ctx := setupTestDB(t)
	s := newService(t, db, newBlockingConverter())

	makePartitioned(t, ctx, db, TableEventsSearch)
	attachDefault(t, ctx, db, TableEventsSearch, false)

	makePartitioned(t, ctx, db, TableEvents)
	attachDefault(t, ctx, db, TableEvents, true)

	states, err := s.TableStates(ctx)
	require.NoError(t, err)

	for _, state := range states {
		switch state.Name {
		case TableEvents:
			require.True(t, state.Adopted, "a partition carrying the bounds constraint is the table the conversion adopted")
		default:
			require.False(t, state.Adopted, "%s has no adopted partition", state.Name)
		}
	}
}

// attachDefault gives a partitioned parent a default partition, with the bounds
// constraint the attach conversion writes when the partition is the original
// table, and without it when it is a provisioned catch-all.
func attachDefault(t *testing.T, ctx context.Context, db database.Database, table Table, adopted bool) {
	t.Helper()

	_, err := db.GetDB().ExecContext(ctx, fmt.Sprintf(`
        CREATE TABLE convoy.%s_default PARTITION OF convoy.%s DEFAULT`, table, table))
	require.NoError(t, err)

	if !adopted {
		return
	}

	_, err = db.GetDB().ExecContext(ctx, fmt.Sprintf(`
        ALTER TABLE convoy.%s_default
            ADD CONSTRAINT %s_default_bounds CHECK (created_at < NOW())`, table, table))
	require.NoError(t, err)
}

// An operation the table is already in the shape for would rewrite nothing while
// holding the single conversion slot, so it is refused before a run exists.
func TestStartRejectsAnOperationTheTableIsAlreadyIn(t *testing.T) {
	db, ctx := setupTestDB(t)
	s := newService(t, db, newBlockingConverter())

	_, err := s.Start(ctx, TableEvents, OperationUnpartition, "user-1")
	require.ErrorIs(t, err, ErrNotPartitioned)
	require.ErrorContains(t, err, "events")

	makePartitioned(t, ctx, db, TableEvents)

	_, err = s.Start(ctx, TableEvents, OperationPartition, "user-1")
	require.ErrorIs(t, err, ErrAlreadyPartitioned)

	// Nothing was recorded, so the next caller still has the slot.
	runs, err := s.List(ctx, 0)
	require.NoError(t, err)
	require.Empty(t, runs)

	accepted := newBlockingConverter()
	close(accepted.release)
	valid := newService(t, db, accepted)

	run, err := valid.Start(ctx, TableEvents, OperationUnpartition, "user-1")
	require.NoError(t, err)
	waitForStatus(t, valid, ctx, run.UID, StatusCompleted)
}

func TestParseTableRejectsAnythingNotConvertible(t *testing.T) {
	for _, table := range Tables() {
		parsed, err := ParseTable(string(table))
		require.NoError(t, err)
		require.Equal(t, table, parsed)
	}

	// projects is a real table, which is the mistake worth catching: only the
	// tables retention deletes from can be converted.
	_, err := ParseTable("projects")
	require.ErrorIs(t, err, ErrUnknownTable)

	_, err = ParseTable("")
	require.ErrorIs(t, err, ErrUnknownTable)
}

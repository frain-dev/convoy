package partitions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
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

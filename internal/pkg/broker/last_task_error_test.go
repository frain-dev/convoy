package broker

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
	"github.com/frain-dev/convoy/worker/task"
)

func TestPostgresBrokerWiresRealTaskErrors(t *testing.T) {
	db := sqlx.NewDb(&sql.DB{}, "postgres")
	stubJobLockDB(t, db)
	deps, err := New(testConfig(config.PostgresQueueProvider), db, log.New("test", log.LevelError))
	require.NoError(t, err)
	_, ok := deps.TaskErrors.(*pgqueue.PostgresQueue)
	require.True(t, ok, "postgres TaskErrors must be the queue, not a no-op stand-in")
}

func TestPostgresLastTaskErrorReturnsPersistedFailedEnqueue(t *testing.T) {
	q, mock := newPostgresQueueWithMock(t)
	persisted := "failed to write to event delivery queue, err: boom:proj_1:evt_1"
	require.Contains(t, persisted, task.ErrFailedToWriteToQueue.Error())

	mock.ExpectQuery(`SELECT last_error\s+FROM convoy\.queue_jobs\s+WHERE queue_name = \$1 AND id = \$2`).
		WithArgs(string(convoy.CreateEventQueue), "create:job").
		WillReturnRows(sqlmock.NewRows([]string{"last_error"}).AddRow(persisted))

	got, err := q.LastTaskError(string(convoy.CreateEventQueue), "create:job")
	require.NoError(t, err)
	require.Equal(t, persisted, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresLastTaskErrorEmptyWhenNoPriorError(t *testing.T) {
	q, mock := newPostgresQueueWithMock(t)

	mock.ExpectQuery(`SELECT last_error\s+FROM convoy\.queue_jobs\s+WHERE queue_name = \$1 AND id = \$2`).
		WithArgs(string(convoy.CreateEventQueue), "create:job").
		WillReturnRows(sqlmock.NewRows([]string{"last_error"}).AddRow(nil))

	got, err := q.LastTaskError(string(convoy.CreateEventQueue), "create:job")
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresLastTaskErrorSurfacesLookupFailure(t *testing.T) {
	q, mock := newPostgresQueueWithMock(t)

	mock.ExpectQuery(`SELECT last_error\s+FROM convoy\.queue_jobs\s+WHERE queue_name = \$1 AND id = \$2`).
		WithArgs(string(convoy.CreateEventQueue), "missing:job").
		WillReturnError(sql.ErrNoRows)

	got, err := q.LastTaskError(string(convoy.CreateEventQueue), "missing:job")
	require.Empty(t, got)
	require.ErrorIs(t, err, pgqueue.ErrTaskNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newPostgresQueueWithMock(t *testing.T) (*pgqueue.PostgresQueue, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	q, err := pgqueue.NewQueue(queue.QueueOptions{
		Names: map[string]int{string(convoy.CreateEventQueue): 1},
		Type:  queue.ProviderPostgres,
		DB:    sqlx.NewDb(db, "sqlmock"),
	})
	require.NoError(t, err)
	return q, mock
}

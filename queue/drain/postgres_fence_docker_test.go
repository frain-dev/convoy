//go:build integration

package drain

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestDockerPostgresStoreFence(t *testing.T) {
	r := dockerRepository(t)
	data, err := os.ReadFile("/sql/1790071201.sql")
	require.NoError(t, err)
	_, err = r.db.ExecContext(t.Context(), strings.Split(string(data), "-- +migrate Down")[0])
	require.NoError(t, err)
	_, err = r.db.ExecContext(t.Context(), `CREATE ROLE queue_executor LOGIN PASSWORD 'disposable-test-only';
		GRANT USAGE ON SCHEMA convoy TO queue_executor;
		GRANT SELECT,INSERT,UPDATE,DELETE ON convoy.queue_jobs,convoy.queue_state TO queue_executor;
		GRANT SELECT ON convoy.queue_store_fence TO queue_executor`)
	require.NoError(t, err)
	fence := PostgresFence{DB: r.db, Scope: "pg-fence", StoreID: "pg-store", ExecutorRole: "queue_executor"}
	require.NoError(t, fence.Bind(t.Context()))
	require.NoError(t, fence.Bind(t.Context()))
	foreign := fence
	foreign.Scope = "another-deployment"
	require.ErrorIs(t, foreign.Bind(t.Context()), ErrConflict)
	sameRole := fence
	sameRole.ExecutorRole = "qa"
	require.Error(t, sameRole.Bind(t.Context()))
	target := Target{Scope: fence.Scope, StoreID: fence.StoreID, Provider: "postgres", ConfigurationRevision: "pg-fence-cfg", Purpose: Quiesce}
	op, err := r.Begin(t.Context(), target, "pg-fence-begin")
	require.NoError(t, err)
	// Start a legacy mutation before the fence. The fence must wait for its
	// transaction to commit rather than claiming an empty barrier too early.
	tx, err := r.db.BeginTxx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.ExecContext(t.Context(), `INSERT INTO convoy.queue_state(queue_name,paused_at) VALUES('fence-existing',NOW())`)
	require.NoError(t, err)
	closing := make(chan error, 1)
	closeCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() { closing <- fence.Close(closeCtx, op) }()
	select {
	case closeErr := <-closing:
		t.Fatalf("fence closed before the accepted transaction settled: %v", closeErr)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, tx.Commit())
	require.NoError(t, <-closing)
	require.NoError(t, fence.Close(t.Context(), op), "closure is idempotent")
	closed, err := fence.Closed(t.Context(), op)
	require.NoError(t, err)
	require.True(t, closed)
	for _, statement := range []string{
		`INSERT INTO convoy.queue_state(queue_name) VALUES('legacy-refill')`,
		`UPDATE convoy.queue_state SET paused_at=NULL WHERE queue_name='fence-existing'`,
		`DELETE FROM convoy.queue_state WHERE queue_name='fence-existing'`,
		`DELETE FROM convoy.queue_jobs WHERE id='missing-job'`,
		`TRUNCATE convoy.queue_jobs`,
	} {
		_, err = r.db.ExecContext(t.Context(), statement)
		require.ErrorContains(t, err, "queue store is fenced")
	}
	executor, err := sqlx.Connect("postgres", "postgres://queue_executor:disposable-test-only@postgres:5432/queue_operations?sslmode=disable&connect_timeout=2")
	require.NoError(t, err)
	defer executor.Close()
	_, err = executor.ExecContext(t.Context(), `INSERT INTO convoy.queue_state(queue_name) VALUES('executor-owned')`)
	require.NoError(t, err, "the dedicated executor must retain access")
	_, err = executor.ExecContext(t.Context(), `UPDATE convoy.queue_store_fence SET closed=FALSE`)
	require.Error(t, err, "the executor cannot reopen admission")
	// Other application tables stay writable while the queue is fenced.
	_, err = r.db.ExecContext(t.Context(), `CREATE TABLE convoy.active_traffic_fixture(id INT PRIMARY KEY); INSERT INTO convoy.active_traffic_fixture VALUES(1)`)
	require.NoError(t, err)
	stale := op
	stale.Epoch--
	require.ErrorIs(t, fence.Open(t.Context(), stale), ErrStale)
	source := op
	source.Purpose = DrainPrevious
	require.ErrorIs(t, fence.Open(t.Context(), source), ErrStale)
	require.ErrorIs(t, fence.Open(t.Context(), op), ErrConflict, "resume must be explicitly requested first")
	op, err = r.Advance(t.Context(), target.Scope, op.ID, op.Revision, Resuming, Evidence{})
	require.NoError(t, err)
	require.NoError(t, fence.Open(t.Context(), op))
	require.NoError(t, fence.Open(t.Context(), op))
	_, err = r.db.ExecContext(t.Context(), `INSERT INTO convoy.queue_state(queue_name) VALUES('resumed-writer')`)
	require.NoError(t, err)
}

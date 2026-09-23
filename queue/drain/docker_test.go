//go:build integration

package drain

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func dockerRepository(t *testing.T) *Repository {
	t.Helper()
	if os.Getenv("QUEUE_OPERATION_DOCKER_QA") != "1" {
		t.Skip("requires disposable operation Docker stack")
	}
	db, err := sqlx.Open("postgres", "postgres://qa:qa@postgres:5432/queue_operations?sslmode=disable&connect_timeout=2")
	require.NoError(t, err)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return NewRepository(db)
}

func TestDockerOperationLifecycle(t *testing.T) {
	r := dockerRepository(t)
	_, err := r.db.ExecContext(t.Context(), `CREATE SCHEMA convoy`)
	require.NoError(t, err)
	data, err := os.ReadFile("/sql/1790071200.sql")
	require.NoError(t, err)
	_, err = r.db.ExecContext(t.Context(), strings.Split(string(data), "-- +migrate Down")[0])
	require.NoError(t, err)
	workerMigration, err := os.ReadFile("/sql/1790071202.sql")
	require.NoError(t, err)
	_, err = r.db.ExecContext(t.Context(), strings.Split(string(workerMigration), "-- +migrate Down")[0])
	require.NoError(t, err)
	executionMigration, err := os.ReadFile("/sql/1790071203.sql")
	require.NoError(t, err)
	_, err = r.db.ExecContext(t.Context(), strings.Split(string(executionMigration), "-- +migrate Down")[0])
	require.NoError(t, err)
	for _, provider := range []string{"redis", "postgres"} {
		for _, purpose := range []Purpose{Quiesce, Drain, DrainPrevious} {
			t.Run(provider+"/"+string(purpose), func(t *testing.T) {
				target := Target{Scope: provider + "-" + string(purpose), StoreID: provider + "-store", Provider: provider, ConfigurationRevision: "cfg-1", Purpose: purpose}
				producer, admitErr := r.Admit(t.Context(), target.Scope, target.StoreID, Producer)
				require.NoError(t, admitErr)
				op, beginErr := r.Begin(t.Context(), target, "begin-1")
				require.NoError(t, beginErr)
				require.Equal(t, int64(1), op.Epoch)
				// Two repository instances model controllers with independent pools
				// and no shared memory. Restart must preserve identity and fencing.
				restarted := dockerRepository(t)
				again, againErr := restarted.Begin(t.Context(), target, "begin-1")
				require.NoError(t, againErr)
				require.Equal(t, op, again)
				_, admitErr = restarted.Admit(t.Context(), target.Scope, target.StoreID, Producer)
				require.ErrorIs(t, admitErr, ErrFenced)
				e := Evidence{Epoch: op.Epoch, ConfigurationRevision: op.ConfigurationRevision, AdmissionClosed: true, WritersFenced: true, ClaimsStopped: true, InspectionSucceeded: true}
				next := Draining
				if purpose == Quiesce {
					next = Quiescing
				}
				_, advanceErr := restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, next, e)
				require.ErrorIs(t, advanceErr, ErrEvidence, "an admitted producer still owns an unsettled receipt")
				require.NoError(t, restarted.Settle(t.Context(), producer))
				require.NoError(t, restarted.Settle(t.Context(), producer), "settling twice is idempotent")
				var retained int
				require.NoError(t, restarted.db.GetContext(t.Context(), &retained, `SELECT count(*) FROM convoy.queue_operation_receipts WHERE id=$1`, producer.ID))
				require.Zero(t, retained, "settled receipts must not grow with traffic")
				op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, next, e)
				require.NoError(t, advanceErr)
				if purpose == DrainPrevious {
					op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Stopping, Evidence{})
					require.NoError(t, advanceErr)
					_, admitErr = r.Admit(t.Context(), target.Scope, target.StoreID, Claim)
					require.ErrorIs(t, admitErr, ErrFenced)
					e.Future = 2
					op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Stopped, e)
					require.NoError(t, advanceErr)
					_, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Resumed, Evidence{})
					require.ErrorIs(t, advanceErr, ErrConflict)
					op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Fencing, Evidence{})
					require.NoError(t, advanceErr)
					op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Draining, e)
					require.NoError(t, advanceErr)
				}
				e.Future = 1
				if purpose != Quiesce {
					_, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Verifying, e)
					require.ErrorIs(t, advanceErr, ErrEvidence, "future work must keep full drain incomplete")
					e.Future = 0
				}
				op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Verifying, e)
				require.NoError(t, advanceErr)
				_, admitErr = r.Admit(t.Context(), target.Scope, target.StoreID, Claim)
				require.ErrorIs(t, admitErr, ErrFenced)
				completed := Drained
				if purpose == Quiesce {
					completed = Quiesced
				}
				op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, completed, e)
				require.NoError(t, advanceErr)
				current, currentErr := r.Current(t.Context(), target.Scope)
				require.NoError(t, currentErr)
				require.Equal(t, op, *current)
				_, admitErr = r.Admit(t.Context(), target.Scope, target.StoreID, Producer)
				require.ErrorIs(t, admitErr, ErrFenced, "completion does not automatically resume admission")
				if purpose != DrainPrevious {
					op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Resuming, Evidence{})
					require.NoError(t, advanceErr)
					e.RuntimeReady, e.AdmissionOpen = true, true
					op, advanceErr = restarted.Advance(t.Context(), target.Scope, op.ID, op.Revision, Resumed, e)
					require.NoError(t, advanceErr)
					newProducer, newErr := r.Admit(t.Context(), target.Scope, target.StoreID, Producer)
					require.NoError(t, newErr)
					require.NoError(t, r.Settle(t.Context(), newProducer))
					replayed, replayErr := restarted.Begin(t.Context(), target, "begin-1")
					require.NoError(t, replayErr)
					require.Equal(t, Resumed, replayed.State)
					fresh, freshErr := r.Begin(t.Context(), target, "begin-2")
					require.NoError(t, freshErr)
					require.Equal(t, op.Epoch+1, fresh.Epoch)
				}
				history, historyErr := r.History(t.Context(), target.Scope, op.ID)
				require.NoError(t, historyErr)
				require.Len(t, history, int(op.Revision))
				for i, entry := range history {
					require.Equal(t, int64(i+1), entry.Revision)
				}
			})
		}
	}
}

func TestDockerConcurrentBeginAndAdmission(t *testing.T) {
	r := dockerRepository(t)
	target := Target{Scope: "concurrent", StoreID: "redis-store", Provider: "redis", ConfigurationRevision: "cfg-1", Purpose: Drain}
	const replicas = 24
	start := make(chan struct{})
	results := make(chan Operation, replicas)
	errorsCh := make(chan error, replicas)
	var wg sync.WaitGroup
	for range replicas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			op, err := r.Begin(t.Context(), target, "same-request")
			if err != nil {
				errorsCh <- err
				return
			}
			results <- op
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		require.NoError(t, err)
	}
	var id string
	for op := range results {
		if id == "" {
			id = op.ID
		}
		require.Equal(t, id, op.ID)
	}
	require.NotEmpty(t, id)
	changed := target
	changed.StoreID = "another-store"
	_, err := r.Begin(t.Context(), changed, "same-request")
	require.ErrorIs(t, err, ErrConflict)
	_, err = r.Begin(t.Context(), target, "different-request")
	require.ErrorIs(t, err, ErrConflict)
	op, err := r.Get(t.Context(), target.Scope, id)
	require.NoError(t, err)
	e := Evidence{Epoch: op.Epoch, ConfigurationRevision: op.ConfigurationRevision, AdmissionClosed: true, WritersFenced: true, ClaimsStopped: true, InspectionSucceeded: true}
	op, err = r.Advance(t.Context(), target.Scope, id, op.Revision, Draining, e)
	require.NoError(t, err)
	start = make(chan struct{})
	receipts := make(chan Receipt, replicas)
	for range replicas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			receipt, admitErr := r.Admit(t.Context(), target.Scope, target.StoreID, Claim)
			if admitErr == nil {
				receipts <- receipt
			} else if !errors.Is(admitErr, ErrFenced) {
				t.Error(admitErr)
			}
		}()
	}
	close(start)
	verified, verifyErr := r.Advance(t.Context(), target.Scope, id, op.Revision, Verifying, e)
	wg.Wait()
	close(receipts)
	count := 0
	for receipt := range receipts {
		count++
		require.NoError(t, r.Settle(t.Context(), receipt))
	}
	if verifyErr == nil {
		require.Zero(t, count, "no claim may race behind a committed verification barrier")
		require.Equal(t, Verifying, verified.State)
	} else {
		require.ErrorIs(t, verifyErr, ErrEvidence)
		require.Positive(t, count)
		_, err = r.Advance(t.Context(), target.Scope, id, op.Revision, Verifying, e)
		require.NoError(t, err)
	}
	_, err = r.Advance(t.Context(), target.Scope, id, op.Revision, Resumed, Evidence{})
	require.ErrorIs(t, err, ErrStale, "a stale controller cannot reopen traffic")
}

func TestDockerOperationDatabaseUnavailable(t *testing.T) {
	r := dockerRepository(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	_, err := r.Admit(ctx, "concurrent", "redis-store", Producer)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrFenced, "connection failure must be surfaced, not mistaken for an observed state")
}

func TestDockerSourceFenceKeepsActiveAdmissionOpen(t *testing.T) {
	r := dockerRepository(t)
	target := Target{Scope: "source-isolation", StoreID: "retired-store", Provider: "redis", ConfigurationRevision: "cfg-1", Purpose: DrainPrevious}
	oldProducer, err := r.Admit(t.Context(), target.Scope, target.StoreID, Producer)
	require.NoError(t, err)
	op, err := r.Begin(t.Context(), target, "source-begin")
	require.NoError(t, err)
	_, err = r.Admit(t.Context(), target.Scope, target.StoreID, Claim)
	require.ErrorIs(t, err, ErrFenced, "source consumers cannot start before the source barrier")
	activeProducer, err := r.Admit(t.Context(), target.Scope, "active-store", Producer)
	require.NoError(t, err)
	defer func() { require.NoError(t, r.Settle(t.Context(), activeProducer)) }()
	e := Evidence{Epoch: op.Epoch, ConfigurationRevision: op.ConfigurationRevision, AdmissionClosed: true, WritersFenced: true, ClaimsStopped: true, InspectionSucceeded: true}
	_, err = r.Advance(t.Context(), target.Scope, op.ID, op.Revision, Draining, e)
	require.ErrorIs(t, err, ErrEvidence)
	require.NoError(t, r.Settle(t.Context(), oldProducer))
	op, err = r.Advance(t.Context(), target.Scope, op.ID, op.Revision, Draining, e)
	require.NoError(t, err, "active receipts must not block a source-only operation")
	claim, err := r.Admit(t.Context(), target.Scope, target.StoreID, Claim)
	require.NoError(t, err)
	_, err = r.Advance(t.Context(), target.Scope, op.ID, op.Revision, Verifying, e)
	require.ErrorIs(t, err, ErrEvidence)
	require.NoError(t, r.Settle(t.Context(), claim))
	op, err = r.Advance(t.Context(), target.Scope, op.ID, op.Revision, Verifying, e)
	require.NoError(t, err)
	oldRevision := op.Revision
	op, err = r.Advance(t.Context(), target.Scope, op.ID, op.Revision, Drained, e)
	require.NoError(t, err)
	repeated, err := r.Advance(t.Context(), target.Scope, op.ID, oldRevision, Drained, e)
	require.NoError(t, err, "a retried completed command is idempotent")
	require.Equal(t, op, repeated)
	_, err = r.Admit(t.Context(), target.Scope, target.StoreID, Producer)
	require.ErrorIs(t, err, ErrFenced)
	activeClaim, err := r.Admit(t.Context(), target.Scope, "active-store", Claim)
	require.NoError(t, err)
	require.NoError(t, r.Settle(t.Context(), activeClaim))
}

func TestDockerOperationDatabaseRecovered(t *testing.T) {
	r := dockerRepository(t)
	op, err := r.Current(t.Context(), "concurrent")
	require.NoError(t, err)
	require.NotNil(t, op)
	require.Equal(t, Verifying, op.State)
	_, err = r.Admit(t.Context(), "concurrent", "redis-store", Producer)
	require.ErrorIs(t, err, ErrFenced)
}

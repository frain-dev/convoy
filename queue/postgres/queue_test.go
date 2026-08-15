package postgres

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	dbpostgres "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/testenv"
)

var testInfra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background(), testenv.WithoutRedis())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}
	testInfra = res
	code := m.Run()
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
	}
	os.Exit(code)
}

func setupQueue(t *testing.T) *PostgresQueue {
	t.Helper()
	conn, err := testInfra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	db := dbpostgres.NewFromConnection(conn)
	q, err := NewQueue(queue.QueueOptions{
		Names: map[string]int{string(convoy.EventQueue): 1},
		Type:  queue.ProviderPostgres,
		DB:    db.GetDB(),
	})
	require.NoError(t, err)
	return q
}

func TestWriteAndClaim(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()

	err := q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("hello"),
	})
	require.NoError(t, err)

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 10)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, id, jobs[0].ID)
	require.Equal(t, []byte("hello"), jobs[0].Payload)
	require.Equal(t, string(convoy.EventProcessor), jobs[0].TaskName)
}

func TestWriteReplacesDuplicateID(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()

	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("first"),
	}))
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("second"),
	}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 10)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, []byte("second"), jobs[0].Payload)
}

func TestWriteDelayIsNotClaimedEarly(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()

	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      ulid.Make().String(),
		Payload: []byte("later"),
		Delay:   2 * time.Second,
	}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 10)
	require.NoError(t, err)
	require.Empty(t, jobs)
}

func TestCompleteRemovesProcessingRow(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("x")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, q.Complete(ctx, jobs[0].ID))

	jobs, err = q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Empty(t, jobs)
}

func TestCompletedCronJobIsIdempotent(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := queue.CronJobID(convoy.SnapshotUsage, time.Now())
	job := &queue.Job{ID: id}

	require.NoError(t, q.Write(ctx, convoy.SnapshotUsage, convoy.EventQueue, job))
	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, q.Complete(ctx, id))

	require.NoError(t, q.Write(ctx, convoy.SnapshotUsage, convoy.EventQueue, job))
	jobs, err = q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Empty(t, jobs)

	var status string
	require.NoError(t, q.db.GetContext(ctx, &status, `SELECT status FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Equal(t, statusCompleted, status)

	counts, err := q.Counts(ctx)
	require.NoError(t, err)
	require.Empty(t, counts)
}

// The archived-jobs cleanup shares a cron minute with other scheduled tasks,
// so it must not erase a tombstone a lagging replica is still deduplicating
// against. Aged tombstones are dropped so the table does not grow forever.
func TestDeleteArchivedKeepsRecentCronTombstones(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := queue.CronJobID(convoy.SnapshotUsage, time.Now())
	job := &queue.Job{ID: id}

	require.NoError(t, q.Write(ctx, convoy.SnapshotUsage, convoy.EventQueue, job))
	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, q.Complete(ctx, id))

	require.NoError(t, q.DeleteArchived(ctx))
	var count int
	require.NoError(t, q.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Equal(t, 1, count)

	// A replica whose clock lagged into the cleanup still cannot re-run the tick.
	require.NoError(t, q.Write(ctx, convoy.SnapshotUsage, convoy.EventQueue, job))
	jobs, err = q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Empty(t, jobs)

	_, err = q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET updated_at = NOW() - make_interval(secs => $2)
		WHERE id = $1`, id, (cronTombstoneRetention + time.Hour).Seconds())
	require.NoError(t, err)

	require.NoError(t, q.DeleteArchived(ctx))
	require.NoError(t, q.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Zero(t, count)
}

func TestDeleteArchivedRemovesNonCronRows(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("x")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, q.Archive(ctx, id, "boom"))

	require.NoError(t, q.DeleteArchived(ctx))
	var count int
	require.NoError(t, q.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Zero(t, count)
}

func TestClaimSkipLockedDoesNotOverlap(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: "a", Payload: []byte("a")}))
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: "b", Payload: []byte("b")}))

	got := make(chan string, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
			require.NoError(t, err)
			require.Len(t, jobs, 1)
			got <- jobs[0].ID
		}()
	}
	wg.Wait()
	close(got)

	ids := map[string]struct{}{}
	for id := range got {
		ids[id] = struct{}{}
	}
	require.Len(t, ids, 2)
}

func TestConcurrentWritesAreDurableOnReturn(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	const jobCount = 64

	start := make(chan struct{})
	errs := make(chan error, jobCount)
	for i := 0; i < jobCount; i++ {
		id := fmt.Sprintf("batched-write-%d", i)
		go func() {
			<-start
			errs <- q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
				ID:      id,
				Payload: []byte(id),
			})
		}()
	}
	close(start)

	for i := 0; i < jobCount; i++ {
		require.NoError(t, <-errs)
	}

	var count int
	require.NoError(t, q.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM convoy.queue_jobs
		WHERE id LIKE 'batched-write-%'`))
	require.Equal(t, jobCount, count)
}

func TestConcurrentCompletesAreDurableOnReturn(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	const jobCount = 64

	for i := 0; i < jobCount; i++ {
		id := fmt.Sprintf("batched-complete-%d", i)
		require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
			ID:      id,
			Payload: []byte(id),
		}))
	}
	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, jobCount)
	require.NoError(t, err)
	require.Len(t, jobs, jobCount)

	start := make(chan struct{})
	errs := make(chan error, jobCount)
	for i := range jobs {
		id := jobs[i].ID
		go func() {
			<-start
			errs <- q.Complete(ctx, id)
		}()
	}
	close(start)

	for i := 0; i < jobCount; i++ {
		require.NoError(t, <-errs)
	}

	var count int
	require.NoError(t, q.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM convoy.queue_jobs
		WHERE id LIKE 'batched-complete-%'`))
	require.Zero(t, count)
}

func TestConcurrentWritesReturnDatabaseError(t *testing.T) {
	q := setupQueue(t)
	require.NoError(t, q.db.Close())

	const jobCount = 4
	start := make(chan struct{})
	errs := make(chan error, jobCount)
	for i := 0; i < jobCount; i++ {
		id := fmt.Sprintf("failed-write-%d", i)
		go func() {
			<-start
			errs <- q.Write(context.Background(), convoy.EventProcessor, convoy.EventQueue, &queue.Job{
				ID:      id,
				Payload: []byte(id),
			})
		}()
	}
	close(start)

	for i := 0; i < jobCount; i++ {
		require.Error(t, <-errs)
	}
}

func TestReclaimStuckMakesRowClaimable(t *testing.T) {
	q := setupQueue(t)
	q.SetStuckTimeout(50 * time.Millisecond)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("stuck")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	_, err = q.db.ExecContext(ctx, `UPDATE convoy.queue_jobs SET claimed_at = NOW() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)

	n, err := q.ReclaimStuck(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	jobs, err = q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, id, jobs[0].ID)
}

func TestHeartbeatKeepsClaimFromBeingReclaimed(t *testing.T) {
	q := setupQueue(t)
	q.SetStuckTimeout(50 * time.Millisecond)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("slow")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	// A handler slower than the lease: without renewal this row is reclaimable.
	_, err = q.db.ExecContext(ctx, `UPDATE convoy.queue_jobs SET claimed_at = NOW() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)

	require.NoError(t, q.Heartbeat(ctx, []string{id}))

	n, err := q.ReclaimStuck(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), n, "a renewed claim must not be reclaimed")

	var status string
	require.NoError(t, q.db.GetContext(ctx, &status, `SELECT status FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Equal(t, statusProcessing, status)
}

func TestHeartbeatIgnoresJobsThisConsumerNoLongerOwns(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("released")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, q.Release(ctx, []string{id}))

	// The row went back to pending, so renewal must not resurrect the claim.
	require.NoError(t, q.Heartbeat(ctx, []string{id}))

	var status string
	require.NoError(t, q.db.GetContext(ctx, &status, `SELECT status FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Equal(t, statusPending, status)

	require.NoError(t, q.Heartbeat(ctx, nil))
}

func TestRetryIncrementsCountAndReleases(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("x")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	require.NoError(t, q.Retry(ctx, id, time.Now().Add(-time.Second), true, "boom"))

	jobs, err = q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, 1, jobs[0].RetryCount)
}

func TestNewQueueRequiresDB(t *testing.T) {
	_, err := NewQueue(queue.QueueOptions{Type: queue.ProviderPostgres})
	require.Error(t, err)
}

func TestCounts(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("x")}))

	counts, err := q.Counts(ctx)
	require.NoError(t, err)
	require.Len(t, counts, 1)
	require.Equal(t, string(convoy.EventQueue), counts[0].QueueName)
	require.Equal(t, int64(1), counts[0].Pending)
	require.Equal(t, int64(0), counts[0].Processing)
}

func TestWriteDoesNotReplaceProcessing(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("first")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	err = q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("second")})
	require.ErrorIs(t, err, ErrJobProcessing)

	counts, err := q.Counts(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), counts[0].Pending)
	require.Equal(t, int64(1), counts[0].Processing)

	require.NoError(t, q.Complete(ctx, id))
	jobs, err = q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Empty(t, jobs)
}

func TestProcessingConflictDoesNotRollBackBatchedSiblings(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	processingID := ulid.Make().String()
	freshID := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      processingID,
		Payload: []byte("processing"),
	}))
	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	results := q.writeBatch([]writeRequest{
		{
			id: processingID, taskName: string(convoy.EventProcessor),
			queueName: string(convoy.EventQueue), payload: []byte("replacement"),
			headers: []byte("{}"), maxRetry: defaultMaxRetry,
		},
		{
			id: freshID, taskName: string(convoy.EventProcessor),
			queueName: string(convoy.EventQueue), payload: []byte("fresh"),
			headers: []byte("{}"), maxRetry: defaultMaxRetry,
		},
	})
	require.ErrorIs(t, results[0], ErrJobProcessing)
	require.NoError(t, results[1])

	var payload []byte
	require.NoError(t, q.db.GetContext(ctx, &payload, `SELECT payload FROM convoy.queue_jobs WHERE id = $1`, freshID))
	require.Equal(t, []byte("fresh"), payload)
}

func TestQueuePriorityCyclePreservesWeights(t *testing.T) {
	q := setupQueue(t)
	q.opts.Names = map[string]int{
		"critical": 5,
		"default":  2,
		"low":      1,
	}

	cycle := q.QueuePriorityCycle()
	require.Len(t, cycle, 8)
	require.Equal(t, 5, countQueue(cycle, "critical"))
	require.Equal(t, 2, countQueue(cycle, "default"))
	require.Equal(t, 1, countQueue(cycle, "low"))
}

func TestClaimHonorsQueuePriorityOrder(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	for i := 0; i < postgresWeightDepth; i++ {
		require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.DefaultQueue, &queue.Job{
			ID:      fmt.Sprintf("default-%d", i),
			Payload: []byte("default"),
		}))
	}
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      "event-priority",
		Payload: []byte("event"),
	}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue), string(convoy.DefaultQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "event-priority", jobs[0].ID)
}

func countQueue(names []string, target string) int {
	count := 0
	for _, name := range names {
		if name == target {
			count++
		}
	}
	return count
}

func TestDeleteEventDeliveriesFromQueueSkipsProcessing(t *testing.T) {
	q := setupQueue(t)
	ctx := context.Background()
	idA := ulid.Make().String()
	idB := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: idA, Payload: []byte("a")}))
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: idB, Payload: []byte("b")}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	require.NoError(t, q.DeleteEventDeliveriesFromQueue(convoy.EventQueue, []string{idA, idB}))

	counts, err := q.Counts(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), counts[0].Pending)
	require.Equal(t, int64(1), counts[0].Processing)
}

// TestCloseReleasesBatchers covers the goroutines NewQueue starts. A long-lived
// process only builds one queue, but the integration suite builds a broker per
// test, and without a shutdown path each build leaked writeConcurrency+1
// goroutines holding the pool.
func TestCloseReleasesBatchers(t *testing.T) {
	before := runtime.NumGoroutine()

	q := setupQueue(t)
	ctx := context.Background()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      ulid.Make().String(),
		Payload: []byte("hello"),
	}))

	require.NoError(t, q.Close())
	require.NoError(t, q.Close(), "Close must be idempotent")

	// Writes offered after Close are refused rather than parked on a channel
	// with no reader.
	err := q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      ulid.Make().String(),
		Payload: []byte("late"),
	})
	require.ErrorIs(t, err, errQueueClosed)

	// The batchers are joined by Close, so anything still resident belongs to
	// the test database handle rather than the queue.
	require.LessOrEqual(t, runtime.NumGoroutine()-before, 4,
		"batcher goroutines should not outlive Close")
}

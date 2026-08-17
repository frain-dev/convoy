package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database/postgres"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
	"github.com/frain-dev/convoy/testenv"
	"github.com/frain-dev/convoy/worker/task"
)

const pgTestTask convoy.TaskName = "postgres.queue.test"

var pgTestEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background(), testenv.WithoutRedis())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}
	pgTestEnv = res
	code := m.Run()
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
	}
	os.Exit(code)
}

func setupPostgresConsumer(t *testing.T) (*Consumer, *pgqueue.PostgresQueue) {
	t.Helper()
	conn, err := pgTestEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	db := postgres.NewFromConnection(conn)
	q, err := pgqueue.NewQueue(queue.QueueOptions{
		Names:              map[string]int{string(convoy.EventQueue): 1},
		Type:               queue.ProviderPostgres,
		DB:                 db.GetDB(),
		PostgresConnString: conn.Config().ConnString(),
	})
	require.NoError(t, err)
	q.SetStuckTimeout(time.Hour)
	t.Cleanup(func() { _ = q.Close() })

	lo := log.New("postgres-queue-test", log.LevelError)
	c, err := NewConsumer(context.Background(), 2, q.Options().Names, NewPostgresConsumerBackend(q), lo, log.LevelError)
	require.NoError(t, err)
	return c, q
}

func waitUntil(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestPostgresConsumerProcessesJob(t *testing.T) {
	c, q := setupPostgresConsumer(t)
	var ran atomic.Bool
	c.RegisterHandlers(pgTestTask, func(ctx context.Context, tk *asynq.Task) error {
		ran.Store(true)
		return nil
	}, nil)
	require.NoError(t, c.Start())
	t.Cleanup(c.Stop)

	id := ulid.Make().String()
	require.NoError(t, q.Write(context.Background(), pgTestTask, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("ok"),
	}))

	waitUntil(t, 3*time.Second, ran.Load)
	waitUntil(t, 3*time.Second, func() bool {
		var n int
		err := q.Options().DB.Get(&n, `SELECT COUNT(*) FROM convoy.queue_jobs WHERE id = $1`, id)
		require.NoError(t, err)
		return n == 0
	})
}

func TestPostgresConsumerRateLimitDoesNotIncrementRetry(t *testing.T) {
	c, q := setupPostgresConsumer(t)
	var attempts atomic.Int32
	c.RegisterHandlers(pgTestTask, func(ctx context.Context, tk *asynq.Task) error {
		n := attempts.Add(1)
		if n == 1 {
			return &task.RateLimitError{Err: errors.New("limited")}
		}
		return nil
	}, nil)
	require.NoError(t, c.Start())
	t.Cleanup(c.Stop)

	id := ulid.Make().String()
	require.NoError(t, q.Write(context.Background(), pgTestTask, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("rl"),
	}))

	waitUntil(t, 8*time.Second, func() bool { return attempts.Load() >= 2 })
}

func TestPostgresConsumerArchivesAfterMaxRetry(t *testing.T) {
	c, q := setupPostgresConsumer(t)
	c.RegisterHandlers(pgTestTask, func(ctx context.Context, tk *asynq.Task) error {
		return errors.New("always")
	}, nil)
	require.NoError(t, c.Start())
	t.Cleanup(c.Stop)

	maxRetry := 0
	id := ulid.Make().String()
	require.NoError(t, q.Write(context.Background(), pgTestTask, convoy.EventQueue, &queue.Job{
		ID:       id,
		Payload:  []byte("dead"),
		MaxRetry: &maxRetry,
	}))

	waitUntil(t, 5*time.Second, func() bool {
		var status string
		err := q.Options().DB.Get(&status, `SELECT status FROM convoy.queue_jobs WHERE id = $1`, id)
		if err != nil {
			return false
		}
		return status == "archived"
	})
}

func TestPostgresConsumerStopWaitsForInFlight(t *testing.T) {
	c, q := setupPostgresConsumer(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	c.RegisterHandlers(pgTestTask, func(ctx context.Context, tk *asynq.Task) error {
		close(entered)
		<-release
		return nil
	}, nil)
	require.NoError(t, c.Start())

	require.NoError(t, q.Write(context.Background(), pgTestTask, convoy.EventQueue, &queue.Job{
		ID:      ulid.Make().String(),
		Payload: []byte("block"),
	}))
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Stop returned before the in-flight handler finished")
	case <-time.After(150 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return after the handler finished")
	}
}

// heartbeatSpyQueue serves one job, then records every lease renewal so a test
// can assert the runner keeps a slow handler's claim alive.
type heartbeatSpyQueue struct {
	job      queue.ClaimedJob
	served   atomic.Bool
	mu       sync.Mutex
	renewals []string
}

func (s *heartbeatSpyQueue) Claim(context.Context, []string, int) ([]queue.ClaimedJob, error) {
	if s.served.Swap(true) {
		return nil, nil
	}
	return []queue.ClaimedJob{s.job}, nil
}

func (s *heartbeatSpyQueue) Heartbeat(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renewals = append(s.renewals, ids...)
	return nil
}

func (s *heartbeatSpyQueue) renewalsFor(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, got := range s.renewals {
		if got == id {
			n++
		}
	}
	return n
}

func (s *heartbeatSpyQueue) Complete(context.Context, string) error                       { return nil }
func (s *heartbeatSpyQueue) Retry(context.Context, string, time.Time, bool, string) error { return nil }
func (s *heartbeatSpyQueue) Archive(context.Context, string, string) error                { return nil }
func (s *heartbeatSpyQueue) Release(context.Context, []string) error                      { return nil }
func (s *heartbeatSpyQueue) ReclaimStuck(context.Context) (int64, error)                  { return 0, nil }

// LeaseTimeout drives the derived renewal interval: 60ms over six renewals is
// 10ms, which the lowered floor in the test below allows through.
func (s *heartbeatSpyQueue) LeaseTimeout() time.Duration { return 60 * time.Millisecond }
func (s *heartbeatSpyQueue) ClaimBatchSize() int         { return 64 }
func (s *heartbeatSpyQueue) PollIdle() time.Duration     { return time.Millisecond }
func (s *heartbeatSpyQueue) Wake() <-chan struct{}       { return nil }

func TestPostgresConsumerRenewsLeaseWhileHandlerRuns(t *testing.T) {
	previous := minHeartbeatInterval
	minHeartbeatInterval = time.Millisecond
	t.Cleanup(func() { minHeartbeatInterval = previous })

	id := ulid.Make().String()
	spy := &heartbeatSpyQueue{job: queue.ClaimedJob{
		ID:        id,
		TaskName:  string(pgTestTask),
		QueueName: string(convoy.EventQueue),
		Payload:   []byte("slow"),
	}}

	entered := make(chan struct{})
	release := make(chan struct{})
	lo := log.New("postgres-heartbeat-test", log.LevelError)
	c, err := NewConsumer(context.Background(), 1, map[string]int{string(convoy.EventQueue): 1},
		NewPostgresConsumerBackend(spy), lo, log.LevelError)
	require.NoError(t, err)
	c.RegisterHandlers(pgTestTask, func(ctx context.Context, tk *asynq.Task) error {
		close(entered)
		<-release
		return nil
	}, nil)
	require.NoError(t, c.Start())

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	waitUntil(t, 3*time.Second, func() bool { return spy.renewalsFor(id) >= 3 })

	close(release)
	waitUntil(t, 3*time.Second, func() bool {
		before := spy.renewalsFor(id)
		time.Sleep(50 * time.Millisecond)
		return spy.renewalsFor(id) == before
	})
	c.Stop()
}

type claimLimitSpyQueue struct {
	claimBatchSize int
}

func (s *claimLimitSpyQueue) Claim(context.Context, []string, int) ([]queue.ClaimedJob, error) {
	return nil, nil
}
func (s *claimLimitSpyQueue) Complete(context.Context, string) error { return nil }
func (s *claimLimitSpyQueue) Retry(context.Context, string, time.Time, bool, string) error {
	return nil
}
func (s *claimLimitSpyQueue) Archive(context.Context, string, string) error { return nil }
func (s *claimLimitSpyQueue) Release(context.Context, []string) error       { return nil }
func (s *claimLimitSpyQueue) ReclaimStuck(context.Context) (int64, error)   { return 0, nil }
func (s *claimLimitSpyQueue) Heartbeat(context.Context, []string) error     { return nil }
func (s *claimLimitSpyQueue) LeaseTimeout() time.Duration                   { return time.Minute }
func (s *claimLimitSpyQueue) ClaimBatchSize() int {
	if s.claimBatchSize == 0 {
		return 64
	}
	return s.claimBatchSize
}
func (s *claimLimitSpyQueue) PollIdle() time.Duration { return time.Millisecond }
func (s *claimLimitSpyQueue) Wake() <-chan struct{}   { return nil }

func TestPostgresConsumerUsesConfiguredPoolAndClaimBatch(t *testing.T) {
	spy := &claimLimitSpyQueue{claimBatchSize: 256}
	backend := &postgresConsumerBackend{queue: spy}

	got, err := backend.newRunner(
		context.Background(),
		256,
		map[string]int{string(convoy.EventQueue): 1},
		asynq.NewServeMux(),
		log.New("postgres-config-test", log.LevelError),
		log.LevelError,
	)
	require.NoError(t, err)

	r := got.(*postgresRunner)
	require.Equal(t, 256, r.poolSize)
	require.Equal(t, 256, r.claimBatchSize)
	require.Equal(t, 256, r.claimLimit)
}

func TestPrioritizeQueueMovesPreferredFirst(t *testing.T) {
	names := []string{"create", "default", "event"}
	require.Equal(t,
		[]string{"event", "create", "default"},
		prioritizeQueue(names, "event"),
	)
	require.Equal(t, names, prioritizeQueue(names, ""))
}

// TestHeartbeatTimeoutNeverExceedsInterval pins the arithmetic the renewal
// guarantee rests on. Heartbeat is called inline and a ticker holds only one
// pending tick, so an attempt allowed to outlive its interval stretches the real
// cadence to the timeout and a live worker gets fewer tries than the lease
// promises. At the 30s configuration floor a fixed 10s timeout left two.
func TestHeartbeatTimeoutNeverExceedsInterval(t *testing.T) {
	for _, lease := range []time.Duration{
		30 * time.Second, // config floor
		90 * time.Second, // default
		10 * time.Minute,
	} {
		interval := heartbeatInterval(lease)
		timeout := heartbeatTimeout(interval)

		require.LessOrEqual(t, timeout, interval,
			"lease %s: an attempt must fit inside its interval", lease)

		attempts := int(lease / interval)
		require.GreaterOrEqual(t, attempts, heartbeatsPerLease-1,
			"lease %s: a worker must survive %d missed renewals", lease, heartbeatsPerLease-1)
	}
}

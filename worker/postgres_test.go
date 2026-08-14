package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
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
		Names: map[string]int{string(convoy.EventQueue): 1},
		Type:  queue.ProviderPostgres,
		DB:    db.GetDB(),
	})
	require.NoError(t, err)
	q.SetStuckTimeout(time.Hour)

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

func TestPrioritizeQueueMovesPreferredFirst(t *testing.T) {
	names := []string{"create", "default", "event"}
	require.Equal(t,
		[]string{"event", "create", "default"},
		prioritizeQueue(names, "event"),
	)
	require.Equal(t, names, prioritizeQueue(names, ""))
}

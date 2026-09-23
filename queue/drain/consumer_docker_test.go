//go:build integration

package drain_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/worker"
)

// These are actual consumers against both brokers, not evidence that the
// deployment-wide admission fence or source ownership split is installed.
func TestDockerConsumerSuspendResume(t *testing.T) {
	if os.Getenv("QUEUE_OPERATION_DOCKER_QA") != "1" {
		t.Skip("requires disposable Docker stack")
	}
	db, err := sqlx.Open("postgres", "postgres://qa:qa@postgres:5432/queue_operations?sslmode=disable&connect_timeout=2")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	for _, name := range []string{"1786696932.sql", "1787000379.sql", "1787168852.sql"} {
		data, err := os.ReadFile("/sql/" + name)
		require.NoError(t, err)
		_, err = db.ExecContext(t.Context(), strings.Split(string(data), "-- +migrate Down")[0])
		require.NoError(t, err)
	}
	for _, provider := range []string{"redis", "postgres"} {
		t.Run(provider, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var q queue.Queuer
			var backend worker.ConsumerBackend
			opts := queue.QueueOptions{Type: provider, DB: db, PostgresTuning: queue.PostgresTuning{ClaimBatchSize: 1}}
			var inspectFuture func() (time.Time, int)
			maxRetry := 7
			if provider == "postgres" {
				pg, err := pgqueue.NewQueue(opts)
				require.NoError(t, err)
				defer pg.Close()
				q, backend = pg, worker.NewPostgresConsumerBackend(pg)
				inspectFuture = func() (time.Time, int) {
					var due time.Time
					var retries int
					err := db.QueryRowContext(ctx, `SELECT run_at,max_retry FROM convoy.queue_jobs WHERE id='consumer-future'`).Scan(&due, &retries)
					require.NoError(t, err)
					return due, retries
				}
			} else {
				rd, err := rdb.NewClient([]string{"redis://redis:6379/0"})
				require.NoError(t, err)
				defer rd.Client().Close()
				opts.RedisClient, opts.RedisAddress = rd, []string{"redis://redis:6379/0"}
				rq := redisqueue.NewQueue(opts)
				q, backend = rq, worker.NewRedisConsumerBackend(opts)
				inspectFuture = func() (time.Time, int) {
					info, readErr := rq.Inspector().GetTaskInfo("consumer-a", "consumer-future")
					require.NoError(t, readErr)
					return info.NextProcessAt, info.MaxRetry
				}
			}
			logger := log.New("consumer-resume-test", log.LevelError)
			names := map[string]int{"consumer-a": 1}
			a, err := worker.NewConsumer(ctx, 1, names, backend, logger, log.LevelError)
			require.NoError(t, err)
			defer a.Stop()
			b, err := worker.NewConsumer(ctx, 1, map[string]int{"consumer-b": 1}, backend, logger, log.LevelError)
			require.NoError(t, err)
			defer b.Stop()
			const taskName convoy.TaskName = "consumer-resume-test"
			entered := make(chan string, 10)
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			a.RegisterHandlers(taskName, func(_ context.Context, task *asynq.Task) error {
				value := string(task.Payload())
				entered <- value
				if value == "inflight" {
					<-release
				}
				return nil
			})
			other := make(chan string, 10)
			b.RegisterHandlers(taskName, func(_ context.Context, task *asynq.Task) error { other <- string(task.Payload()); return nil })
			require.NoError(t, a.Start())
			require.NoError(t, b.Start())
			write := func(name, id string, delay time.Duration) {
				require.NoError(t, q.Write(ctx, taskName, convoy.QueueName(name), &queue.Job{ID: id, Payload: []byte(id), Delay: delay, MaxRetry: &maxRetry}))
			}
			receive := func(ch <-chan string, want string) {
				t.Helper()
				select {
				case got := <-ch:
					require.Equal(t, want, got)
				case <-time.After(10 * time.Second):
					t.Fatalf("consumer did not process %s", want)
				}
			}
			write("consumer-a", "consumer-future", 24*time.Hour)
			due, retries := inspectFuture()
			write("consumer-a", "inflight", 0)
			receive(entered, "inflight")
			suspended := make(chan error, 1)
			go func() { suspended <- a.Suspend() }()
			settleWait := 100 * time.Millisecond
			if provider == "redis" {
				// Exceed Asynq's normal eight-second shutdown deadline. Suspend
				// must not abandon a live handler and start a second pool over it.
				settleWait = 9 * time.Second
			}
			select {
			case suspendErr := <-suspended:
				t.Fatalf("suspend returned before the active handler settled: %v", suspendErr)
			case <-time.After(settleWait):
			}
			unblock()
			select {
			case suspendErr := <-suspended:
				require.NoError(t, suspendErr)
			case <-time.After(15 * time.Second):
				t.Fatal("consumer did not suspend")
			}
			// The broker-owned Redis connection must still support both writers
			// and the other consumer after A's complete shutdown.
			write("consumer-b", "other-worker", 0)
			receive(other, "other-worker")
			write("consumer-a", "after-suspend", 0)
			select {
			case got := <-entered:
				t.Fatalf("suspended consumer claimed %s", got)
			case <-time.After(1200 * time.Millisecond):
			}
			delete(names, "consumer-a")
			names["wrong-queue"] = 1
			var wg sync.WaitGroup
			for range 8 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if startErr := a.Start(); startErr != nil {
						t.Error(startErr)
					}
				}()
			}
			wg.Wait()
			receive(entered, "after-suspend")
			require.NoError(t, a.Suspend())
			require.NoError(t, a.Suspend(), "suspend is idempotent")
			write("consumer-a", "second-resume", 0)
			require.NoError(t, a.Start())
			receive(entered, "second-resume")
			actualDue, actualRetries := inspectFuture()
			require.Equal(t, due, actualDue)
			require.Equal(t, retries, actualRetries)
			a.Stop()
			require.Error(t, a.Start(), "final shutdown cannot be undone")
		})
	}
}

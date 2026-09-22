//go:build integration

package inventory_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	redislib "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/inventory"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
)

// These fixtures run only in the isolated Compose network described in the QA
// document. They never inherit CONVOY_DB/REDIS configuration from the host.
func dockerConfig(t *testing.T) config.Configuration {
	t.Helper()
	if os.Getenv("QUEUE_INVENTORY_DOCKER_QA") != "1" {
		t.Skip("requires isolated Docker QA stack")
	}
	return config.Configuration{QueueProvider: config.RedisQueueProvider, PreviousQueueProvider: config.PostgresQueueProvider, QueueStoreScope: "queue-inventory-docker-qa",
		Redis:    config.RedisConfiguration{Scheme: "redis", Host: "redis", Port: 6379},
		Database: config.DatabaseConfiguration{Scheme: "postgres", Host: "postgres", Port: 5432, Database: "queue_inventory", Username: "qa", Password: "qa", Options: "sslmode=disable"},
	}
}

func TestDockerInventoryBothProviderDirections(t *testing.T) {
	cfg := dockerConfig(t)
	db, err := sqlx.Connect("postgres", cfg.Database.BuildDsn())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS convoy")
	require.NoError(t, err)
	// Apply the real queue migrations, never a hand-maintained test schema.
	for _, name := range []string{"1786696932.sql", "1787000379.sql", "1787168852.sql"} {
		data, readErr := os.ReadFile(filepath.Join("/sql", name))
		require.NoError(t, readErr)
		up := strings.Split(string(data), "-- +migrate Down")[0]
		_, err = db.Exec(up)
		require.NoError(t, err)
	}
	q, err := pgqueue.NewQueue(queue.QueueOptions{DB: db, Type: "postgres"})
	require.NoError(t, err)
	defer q.Close()
	redisClient := asynq.NewClient(asynq.RedisClientOpt{Addr: "redis:6379"})
	defer redisClient.Close()
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: "redis:6379"})
	defer inspector.Close()
	suffix := time.Now().UTC().Format("150405.000000000")
	redisQueue := "inventory-" + strings.ReplaceAll(suffix, ".", "-")
	pgQueue := convoy.QueueName(redisQueue)
	enqueue := func(id string, delay time.Duration) {
		require.NoError(t, q.Write(t.Context(), convoy.EventProcessor, pgQueue, &queue.Job{ID: id + suffix, Payload: []byte(`{"fixture":true}`), Delay: delay}))
	}
	enqueue("pending", 0)
	enqueue("future", time.Hour)
	enqueue("archive", 0)
	require.NoError(t, q.ArchiveTask(t.Context(), string(pgQueue), "archive"+suffix))
	for _, entry := range []struct {
		id      string
		options []asynq.Option
	}{
		{"pending", nil}, {"future", []asynq.Option{asynq.ProcessIn(time.Hour)}}, {"archive", nil}, {"group", []asynq.Option{asynq.Group("fixture")}},
	} {
		opts := append([]asynq.Option{asynq.Queue(redisQueue), asynq.TaskID(entry.id + suffix)}, entry.options...)
		_, err = redisClient.Enqueue(asynq.NewTask("fixture", []byte(`{"fixture":true}`)), opts...)
		require.NoError(t, err)
	}
	require.NoError(t, inspector.ArchiveTask(redisQueue, "archive"+suffix))
	require.NoError(t, inspector.PauseQueue(redisQueue))
	require.NoError(t, q.PauseQueue(t.Context(), string(pgQueue)))
	for _, active := range []config.QueueProvider{config.RedisQueueProvider, config.PostgresQueueProvider} {
		t.Run(string(active)+"-active", func(t *testing.T) {
			cfg.QueueProvider = active
			cfg.PreviousQueueProvider = config.RedisQueueProvider
			if active == config.RedisQueueProvider {
				cfg.PreviousQueueProvider = config.PostgresQueueProvider
			}
			inv, err := inventory.Configured(cfg, db)
			require.NoError(t, err)
			for pass := 0; pass < 2; pass++ {
				report := inv.Inspect(t.Context())
				require.Len(t, report, 2)
				for _, store := range report {
					require.Equal(t, "connected", store.Connection)
					require.Empty(t, store.Actions)
					require.True(t, store.Visible)
					var found *inventory.Queue
					for _, row := range store.Snapshot.Queues {
						if row.Name == redisQueue {
							copy := row
							found = &copy
						}
					}
					require.NotNil(t, found)
					require.True(t, found.Paused)
					require.Equal(t, int64(1), found.Pending)
					require.Equal(t, int64(1), found.Archived)
					if store.Provider == "redis" {
						require.Equal(t, int64(1), found.Scheduled)
						require.Equal(t, int64(1), found.Aggregating)
					} else {
						require.Equal(t, int64(1), found.Future)
					}
				}
			}
		})
	}
	// A future retry retains its original deadline/budget after repeated reads.
	future, err := inspector.GetTaskInfo(redisQueue, "future"+suffix)
	require.NoError(t, err)
	require.True(t, future.NextProcessAt.After(time.Now().Add(50*time.Minute)))
	require.Zero(t, future.Retried)
	var pgBudget int
	require.NoError(t, db.Get(&pgBudget, "SELECT retry_count FROM convoy.queue_jobs WHERE id=$1", "future"+suffix))
	require.Zero(t, pgBudget)
	var schedulerCount int
	require.NoError(t, db.Get(&schedulerCount, "SELECT COUNT(*) FROM convoy.queue_scheduler_entries"))
	require.Zero(t, schedulerCount)
	servers, err := inspector.Servers()
	require.NoError(t, err)
	require.Empty(t, servers, "inspection must not start consumers")
	// Fresh registration reconstructs the previous store without in-memory history.
	restarted, err := inventory.Configured(cfg, db)
	require.NoError(t, err)
	require.True(t, restarted.Inspect(t.Context())[1].Visible)
}

func TestDockerInventoryDisconnectedPreviousStore(t *testing.T) {
	cfg := dockerConfig(t)
	stopped := os.Getenv("QUEUE_INVENTORY_STOPPED_PROVIDER")
	if stopped != "redis" && stopped != "postgres" {
		t.Skip("requires one stopped disposable store")
	}
	db, err := sqlx.Open("postgres", cfg.Database.BuildDsn())
	require.NoError(t, err)
	defer db.Close()
	cfg.PreviousQueueProvider = config.QueueProvider(stopped)
	cfg.QueueProvider = config.RedisQueueProvider
	if stopped == "redis" {
		cfg.QueueProvider = config.PostgresQueueProvider
	}
	inv, err := inventory.Configured(cfg, db)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	rows := inv.Inspect(ctx)
	require.Len(t, rows, 2)
	require.Equal(t, "connected", rows[0].Connection)
	require.Equal(t, "unknown", rows[1].Connection)
	require.True(t, rows[1].Visible)
	require.Nil(t, rows[1].Snapshot)
	require.Empty(t, rows[1].Actions)
}

func TestDockerInventoryReadDeadline(t *testing.T) {
	cfg := dockerConfig(t)
	cfg.PreviousQueueProvider = ""
	client := redislib.NewClient(&redislib.Options{Addr: "redis:6379"})
	defer client.Close()
	require.NoError(t, client.Do(t.Context(), "CLIENT", "PAUSE", 2000, "ALL").Err())
	inv, err := inventory.Configured(cfg, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	rows := inv.Inspect(ctx)
	require.Less(t, time.Since(started), time.Second, "inspection must honor its request deadline")
	require.Equal(t, "unknown", rows[0].Connection)
	require.Nil(t, rows[0].Snapshot)
	// Let Redis finish its test pause before the next matrix leg.
	require.NoError(t, client.Ping(t.Context()).Err())
}

package redis

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/queue"
)

func setupInspectorQueue(t *testing.T) *RedisQueue {
	t.Helper()
	client, err := testInfra.NewRedisClient(t, 0)
	require.NoError(t, err)
	require.NoError(t, client.FlushDB(t.Context()).Err())

	dsn := "redis://" + client.Options().Addr
	rdbClient, err := rdb.NewClient([]string{dsn})
	require.NoError(t, err)

	return NewQueue(queue.QueueOptions{
		Names:        map[string]int{string(convoy.EventQueue): 1},
		Type:         queue.ProviderRedis,
		RedisClient:  rdbClient,
		RedisAddress: []string{dsn},
	})
}

func TestRedisInspectorStatsAndTasks(t *testing.T) {
	q := setupInspectorQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)
	id := ulid.Make().String()

	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("secret-payload"),
	}))

	stats, err := q.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, queue.ProviderRedis, stats.Provider)
	require.Contains(t, stats.Statuses, queue.StatusRetry, "redis has states postgres does not, and must report them")
	require.Len(t, stats.Queues, 1)
	require.Equal(t, name, stats.Queues[0].Queue)
	require.Equal(t, int64(1), stats.Queues[0].Counts[queue.StatusPending])

	page, err := q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 1})
	require.NoError(t, err)
	require.Len(t, page.Tasks, 1)
	require.Equal(t, id, page.Tasks[0].ID)
	require.Equal(t, name, page.Tasks[0].Queue)
	require.Equal(t, string(convoy.EventProcessor), page.Tasks[0].TaskName)
	require.Equal(t, queue.StatusPending, page.Tasks[0].Status)
	require.False(t, page.HasNext)

	// The drill-down is an operational view, not an event browser: the payload
	// must not leave the queue through it.
	require.NotContains(t, page.Tasks[0].LastError, "secret-payload")

	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusArchived, Page: 1})
	require.NoError(t, err)
	require.Empty(t, page.Tasks)
}

func TestRedisInspectorTasksRejectsBadFilters(t *testing.T) {
	q := setupInspectorQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)

	_, err := q.Tasks(ctx, queue.TaskFilter{Status: queue.StatusPending, Page: 1})
	require.ErrorIs(t, err, queue.ErrQueueRequired)

	// Completed and aggregating are asynq states Convoy does not use, so the
	// page must reject them rather than answer with an empty list.
	for _, status := range []string{"completed", "aggregating", "bogus", ""} {
		_, err = q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: status, Page: 1})
		require.ErrorIs(t, err, queue.ErrUnknownTaskStatus, status)
	}

	for _, page := range []int{0, -1, queue.MaxTaskPage + 1} {
		_, err = q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: page})
		require.ErrorIs(t, err, queue.ErrInvalidPage)
	}
}

func TestRedisInspectorArchiveAndRetry(t *testing.T) {
	q := setupInspectorQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)
	id := ulid.Make().String()

	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("x"),
	}))

	// The page offers exactly the transitions asynq accepts, so an operator is
	// never shown a button the broker rejects.
	page, err := q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 1})
	require.NoError(t, err)
	require.Equal(t, []string{queue.ActionArchive}, page.Tasks[0].Actions)

	require.NoError(t, q.ArchiveTask(ctx, name, id))
	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusArchived, Page: 1})
	require.NoError(t, err)
	require.Len(t, page.Tasks, 1)
	require.Equal(t, id, page.Tasks[0].ID)
	require.Equal(t, []string{queue.ActionRetry}, page.Tasks[0].Actions)

	require.NoError(t, q.RetryTask(ctx, name, id))
	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 1})
	require.NoError(t, err)
	require.Len(t, page.Tasks, 1)
	require.Equal(t, id, page.Tasks[0].ID)

	// A task or queue asynq cannot find is the one absence it proves, so it
	// maps to not-found; anything else stays an unclassified broker error.
	require.ErrorIs(t, q.RetryTask(ctx, name, "no-such-task"), queue.ErrTaskNotFound)
	require.ErrorIs(t, q.ArchiveTask(ctx, name, "no-such-task"), queue.ErrTaskNotFound)
	require.ErrorIs(t, q.RetryTask(ctx, "no-such-queue", id), queue.ErrTaskNotFound)
}

func TestRedisInspectorPaginates(t *testing.T) {
	q := setupInspectorQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)

	for range queue.TasksPerPage + 1 {
		require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
			ID:      ulid.Make().String(),
			Payload: []byte("x"),
		}))
	}

	first, err := q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 1})
	require.NoError(t, err)
	require.Len(t, first.Tasks, queue.TasksPerPage)
	require.True(t, first.HasNext)

	// Asynq derives its offset from the page size, so the page size must stay
	// exactly TasksPerPage or the second page skips rows.
	second, err := q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 2})
	require.NoError(t, err)
	require.Len(t, second.Tasks, 1)
	require.False(t, second.HasNext)

	seen := map[string]bool{}
	for _, task := range append(first.Tasks, second.Tasks...) {
		require.False(t, seen[task.ID], "task %s appeared on both pages", task.ID)
		seen[task.ID] = true
	}
	require.Len(t, seen, queue.TasksPerPage+1)
}

//go:build integration

package drain

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/stretchr/testify/require"
)

func TestDockerExecutionExclusion(t *testing.T) {
	r := dockerRepository(t)
	entered, release := make(chan struct{}), make(chan struct{})
	outcome := make(chan error, 1)
	go func() {
		outcome <- r.Execute(t.Context(), "execution", "redis", "operational", "one", func(context.Context) error { close(entered); <-release; return nil })
	}()
	<-entered
	require.ErrorIs(t, r.Execute(t.Context(), "execution", "postgres", "operational", "one", func(context.Context) error { t.Error("concurrent executor ran"); return nil }), ErrFenced)
	close(release)
	require.NoError(t, <-outcome)
	require.NoError(t, r.Execute(t.Context(), "execution", "postgres", "operational", "one", func(context.Context) error { t.Error("completed task ran twice"); return nil }))
	deferral := errors.New("retry policy marker")
	require.Same(t, deferral, r.Execute(t.Context(), "execution", "redis", "operational", "two", func(context.Context) error { return deferral }))
	require.ErrorIs(t, r.Execute(t.Context(), "execution", "postgres", "operational", "two", func(context.Context) error { t.Error("retry changed owner"); return nil }), ErrFenced)
	require.NoError(t, r.Execute(t.Context(), "execution", "redis", "operational", "two", func(context.Context) error { return nil }))
	// A retry task must consult the same durable ownership after the original
	// task loses its session; sharing only the advisory lock is insufficient.
	require.ErrorIs(t, r.Execute(t.Context(), "execution", "redis", string(convoy.EventProcessor), "ambiguous-delivery", func(context.Context) error { return ErrEvidence }), ErrEvidence)
	for _, store := range []string{"redis", "postgres"} {
		require.ErrorIs(t, r.Execute(t.Context(), "execution", store, string(convoy.RetryEventProcessor), "ambiguous-delivery", func(context.Context) error { t.Error("uncertain delivery replayed as a retry"); return nil }), ErrFenced)
	}
	// Initial dispatch and retry are separate tasks but share the delivery lock.
	entered, release = make(chan struct{}), make(chan struct{})
	go func() {
		outcome <- r.Execute(t.Context(), "execution", "redis", string(convoy.EventProcessor), "delivery", func(ctx context.Context) error {
			require.True(t, queue.AuthoritativeReads(ctx))
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	require.ErrorIs(t, r.Execute(t.Context(), "execution", "postgres", string(convoy.RetryEventProcessor), "delivery", func(context.Context) error { t.Error("concurrent retry dispatched"); return nil }), ErrFenced)
	close(release)
	require.NoError(t, <-outcome)
	_, err := r.db.ExecContext(t.Context(), `INSERT INTO convoy.queue_execution_ownership(scope,task_name,task_id,store_id,state) VALUES('execution','operational','crashed','redis','running')`)
	require.NoError(t, err)
	require.ErrorIs(t, r.Execute(t.Context(), "execution", "redis", "operational", "crashed", func(context.Context) error { t.Error("uncertain task replayed"); return nil }), ErrFenced)
}

func TestDockerWorkerAuthorityLoss(t *testing.T) {
	r := dockerRepository(t)
	gate := &Gate{Repository: r, Scope: "worker-authority", StoreID: "source"}
	consumer := &controlledConsumer{}
	runtime := &Runtime{Repository: r, Scope: gate.Scope, StoreID: gate.StoreID, ConfigurationRevision: "cfg", Consumer: consumer, Gate: gate}
	require.NoError(t, runtime.Reconcile(t.Context()))
	require.NoError(t, gate.Run(t.Context(), Claim, func(context.Context) error { return nil }))
	oldID := gate.worker()
	var pid int
	require.NoError(t, runtime.session.GetContext(t.Context(), &pid, `SELECT pg_backend_pid()`))
	_, err := r.db.ExecContext(t.Context(), `SELECT pg_terminate_backend($1)`, pid)
	require.NoError(t, err)
	require.ErrorIs(t, gate.Run(t.Context(), Claim, func(context.Context) error { t.Error("revoked worker ran"); return nil }), ErrFenced)
	workers, err := r.Workers(t.Context(), gate.Scope)
	require.NoError(t, err)
	require.Empty(t, workers, "database proves the old authority is gone")
	require.Error(t, runtime.Reconcile(t.Context()))
	require.False(t, consumer.running)
	require.NoError(t, runtime.Reconcile(t.Context()))
	require.NotEqual(t, oldID, gate.worker())
	require.NoError(t, gate.Run(t.Context(), Claim, func(context.Context) error { return nil }))
	require.NoError(t, runtime.Close(t.Context()))
}

package drain

import (
	"context"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/stretchr/testify/require"
	"testing"
)

type routingRecorder struct {
	queue.Queuer
	writes int
}

func (r *routingRecorder) Write(context.Context, convoy.TaskName, convoy.QueueName, *queue.Job) error {
	r.writes++
	return nil
}
func (r *routingRecorder) WriteWithoutTimeout(ctx context.Context, task convoy.TaskName, name convoy.QueueName, job *queue.Job) error {
	return r.Write(ctx, task, name, job)
}

func TestRouteDescendantsPreservesAcceptedStore(t *testing.T) {
	producer, active, previous := &routingRecorder{}, &routingRecorder{}, &routingRecorder{}
	routed := RouteDescendants(producer, "test", map[string]queue.Queuer{"active": active, "previous": previous})
	require.NoError(t, routed.Write(context.Background(), convoy.MetaEventProcessor, convoy.MetaEventQueue, &queue.Job{}))
	for _, store := range []string{"active", "previous"} {
		ctx := context.WithValue(context.Background(), admissionKey{}, &admittedWork{gate: &Gate{Scope: "test", StoreID: store}, preserveExisting: true})
		require.NoError(t, routed.Write(ctx, convoy.MetaEventProcessor, convoy.MetaEventQueue, &queue.Job{}))
		require.NoError(t, routed.WriteWithoutTimeout(ctx, convoy.MetaEventProcessor, convoy.MetaEventQueue, &queue.Job{}))
	}
	require.Equal(t, 1, producer.writes)
	require.Equal(t, 2, active.writes)
	require.Equal(t, 2, previous.writes)
}

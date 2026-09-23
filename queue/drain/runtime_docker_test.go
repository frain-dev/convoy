//go:build integration

package drain

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/queue/inventory"
	"github.com/stretchr/testify/require"
)

type controlledConsumer struct{ running bool }

func (c *controlledConsumer) Start() error   { c.running = true; return nil }
func (c *controlledConsumer) Suspend() error { c.running = false; return nil }

// These lifecycle doubles deliberately test only coordination. Store fencing
// and real consumer shutdown have separate real-provider acceptance tests.
type controlledFence struct{ closed bool }

func (f *controlledFence) Bind(context.Context) error                      { return nil }
func (f *controlledFence) Check(context.Context) error                     { return nil }
func (f *controlledFence) Close(context.Context, Operation) error          { f.closed = true; return nil }
func (f *controlledFence) Open(context.Context, Operation) error           { f.closed = false; return nil }
func (f *controlledFence) Closed(context.Context, Operation) (bool, error) { return f.closed, nil }

type controlledReader struct{ counts inventory.Counts }

func (r *controlledReader) Inspect(context.Context) (inventory.Snapshot, error) {
	return inventory.Snapshot{ObservedAt: time.Now(), Queues: []inventory.Queue{{Name: "queue", Counts: r.counts}}}, nil
}

func TestDockerRuntimeCoordination(t *testing.T) {
	r := dockerRepository(t)
	for _, purpose := range []Purpose{Quiesce, Drain} {
		t.Run(string(purpose), func(t *testing.T) {
			scope := "runtime-" + string(purpose)
			reader := &controlledReader{counts: inventory.Counts{Future: 1}}
			fence := &controlledFence{}
			controller := &Controller{Repository: r, Target: Target{Scope: scope, StoreID: "store", Provider: "redis", ConfigurationRevision: "cfg"}, Fence: fence, Reader: reader}
			consumers := []*controlledConsumer{{}, {}}
			runtimes := []*Runtime{}
			for _, consumer := range consumers {
				runtime := &Runtime{Repository: r, Scope: scope, StoreID: "store", ConfigurationRevision: "cfg", Consumer: consumer}
				require.NoError(t, runtime.Reconcile(t.Context()))
				runtimes = append(runtimes, runtime)
			}
			op, err := controller.Begin(t.Context(), purpose, "start", "cfg")
			require.NoError(t, err)
			require.NoError(t, controller.Reconcile(t.Context()))
			// One acknowledgement is never enough for completion.
			require.NoError(t, runtimes[0].Reconcile(t.Context()))
			require.NoError(t, controller.Reconcile(t.Context()))
			current, err := r.Current(t.Context(), scope)
			require.NoError(t, err)
			require.NotEqual(t, Quiesced, current.State)
			require.NotEqual(t, Drained, current.State)
			for range 3 {
				for _, runtime := range runtimes {
					require.NoError(t, runtime.Reconcile(t.Context()))
				}
				require.NoError(t, controller.Reconcile(t.Context()))
			}
			current, err = r.Current(t.Context(), scope)
			require.NoError(t, err)
			if purpose == Quiesce {
				require.Equal(t, Quiesced, current.State)
			} else {
				require.Equal(t, Draining, current.State, "future work prevents an empty certificate")
				reader.counts = inventory.Counts{}
				for range 4 {
					for _, runtime := range runtimes {
						require.NoError(t, runtime.Reconcile(t.Context()))
					}
					require.NoError(t, controller.Reconcile(t.Context()))
				}
				current, err = r.Current(t.Context(), scope)
				require.NoError(t, err)
				require.Equal(t, Drained, current.State)
			}
			require.True(t, fence.closed)
			for _, consumer := range consumers {
				require.False(t, consumer.running)
			}
			_, err = controller.Command(t.Context(), op.ID, current.Revision, "resume_traffic")
			require.NoError(t, err)
			require.ErrorIs(t, controller.Reconcile(t.Context()), ErrEvidence)
			require.True(t, fence.closed, "resume waits for worker readiness")
			for _, runtime := range runtimes {
				require.NoError(t, runtime.Reconcile(t.Context()))
			}
			require.NoError(t, controller.Reconcile(t.Context()))
			require.False(t, fence.closed)
			current, err = r.Current(t.Context(), scope)
			require.NoError(t, err)
			require.Equal(t, Resumed, current.State)
			for _, runtime := range runtimes {
				require.NoError(t, runtime.Close(t.Context()))
			}
		})
	}
}

func TestDockerCompletedOperationConfigurationChange(t *testing.T) {
	r := dockerRepository(t)
	scope := "completed-configuration-change"
	reader := &controlledReader{}
	fence := &controlledFence{}
	controller := &Controller{Repository: r, Target: Target{Scope: scope, StoreID: "store", Provider: "redis", ConfigurationRevision: "old"}, Fence: fence, Reader: reader, Previous: true}
	runtime := &Runtime{Repository: r, Scope: scope, StoreID: "store", ConfigurationRevision: "old", Consumer: &controlledConsumer{}, Previous: true}
	require.NoError(t, runtime.Reconcile(t.Context()))
	_, err := controller.Begin(t.Context(), DrainPrevious, "first", "old")
	require.NoError(t, err)
	for range 6 {
		require.NoError(t, runtime.Reconcile(t.Context()))
		require.NoError(t, controller.Reconcile(t.Context()))
	}
	op, err := r.Current(t.Context(), scope)
	require.NoError(t, err)
	require.Equal(t, Drained, op.State)
	require.NoError(t, runtime.Close(t.Context()))
	controller.Target.ConfigurationRevision = "new"
	runtime = &Runtime{Repository: r, Scope: scope, StoreID: "store", ConfigurationRevision: "new", Consumer: &controlledConsumer{}, Previous: true}
	require.NoError(t, runtime.Reconcile(t.Context()))
	defer runtime.Close(t.Context())
	status, err := controller.Status(t.Context())
	require.NoError(t, err)
	require.Empty(t, status.Blockers)
	require.Contains(t, status.Actions, "drain_previous")
	next, err := controller.Begin(t.Context(), DrainPrevious, "second", "new")
	require.NoError(t, err)
	require.Greater(t, next.Epoch, op.Epoch)
}

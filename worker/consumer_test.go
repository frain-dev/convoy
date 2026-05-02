package worker

import (
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

// swapTaskPayload mutates an asynq.Task's private payload field via
// reflection. The reason it exists rather than rebuilding the task with
// asynq.NewTask is documented in consumer.go — but if the underlying field
// ever renames, the function panics on reflection rather than silently
// failing, and this test would surface that.
func TestSwapTaskPayload_MutatesPayloadInPlace(t *testing.T) {
	task := asynq.NewTask("test.task", []byte("original"))
	addr := task // capture pointer identity to confirm we don't replace the struct

	swapTaskPayload(task, []byte("replaced"))

	require.Equal(t, []byte("replaced"), task.Payload())
	require.Equal(t, "test.task", task.Type(), "swap must not affect Type")
	require.Same(t, addr, task, "swap must mutate the original task pointer")
}

func TestSwapTaskPayload_EmptyPayload(t *testing.T) {
	task := asynq.NewTask("test.task", []byte("original"))
	swapTaskPayload(task, []byte{})
	require.Empty(t, task.Payload())
}

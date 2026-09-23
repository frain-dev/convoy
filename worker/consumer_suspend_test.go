package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

func TestSettlingHandlerWaitsForAcceptedWorkAndRejectsLateGeneration(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	h := &settlingHandler{next: asynq.HandlerFunc(func(context.Context, *asynq.Task) error {
		calls.Add(1)
		close(entered)
		<-release
		return nil
	})}
	done := make(chan error, 1)
	go func() { done <- h.ProcessTask(t.Context(), asynq.NewTask("test", nil)) }()
	<-entered
	settled := make(chan struct{})
	go func() { h.closeAndWait(); close(settled) }()
	select {
	case <-settled:
		t.Fatal("returned before accepted work settled")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-done)
	<-settled
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, h.ProcessTask(ctx, asynq.NewTask("late", nil)), context.Canceled)
	require.Equal(t, int64(1), calls.Load(), "a late task must never enter the old handler generation")
	h.closeAndWait()
}

package drain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

type removalRecorder struct {
	queue.Queuer
	calls int
	err   error
}

func (r *removalRecorder) DeleteEventDeliveriesFromQueue(convoy.QueueName, []string) error {
	r.calls++
	return r.err
}

func TestQueueReplacementSharesItsParentAdmission(t *testing.T) {
	gate := &Gate{Scope: "scope", StoreID: "store"}
	underlying := &removalRecorder{}
	guarded := gate.GuardQueue(underlying).(*guardedQueue)
	parent := &admittedWork{gate: gate, accepting: true}
	ctx := context.WithValue(t.Context(), admissionKey{}, parent)

	require.NoError(t, guarded.DeleteEventDeliveriesFromQueueContext(ctx, convoy.EventQueue, []string{"one"}))
	require.Equal(t, 1, underlying.calls)

	underlying.err = errors.New("removal outcome unknown")
	require.ErrorIs(t, guarded.DeleteEventDeliveriesFromQueueContext(ctx, convoy.EventQueue, []string{"two"}), underlying.err)
	require.True(t, parent.uncertain, "failed queue replacement must retain the outer receipt")

	parent.accepting = false
	require.ErrorIs(t, guarded.DeleteEventDeliveriesFromQueueContext(ctx, convoy.EventQueue, []string{"three"}), ErrFenced)
	require.Equal(t, 2, underlying.calls)
}

package drain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testOperation(p Purpose, state State) Operation {
	return Operation{Target: Target{Scope: "test", StoreID: "store", Provider: "redis", ConfigurationRevision: "config-1", Purpose: p}, Epoch: 3, Revision: 1, State: state}
}

func completeEvidence() Evidence {
	return Evidence{Epoch: 3, ConfigurationRevision: "config-1", AdmissionClosed: true, WritersFenced: true, ClaimsStopped: true, InspectionSucceeded: true}
}

func TestCompletionRequiresFreshBarrierAndNoUnresolvedWork(t *testing.T) {
	for _, purpose := range []Purpose{Drain, DrainPrevious} {
		for name, change := range map[string]func(*Evidence){
			"future retry":            func(e *Evidence) { e.Future = 1 },
			"archived":                func(e *Evidence) { e.Archived = 1 },
			"pending":                 func(e *Evidence) { e.Pending = 1 },
			"processing":              func(e *Evidence) { e.Processing = 1 },
			"aggregation":             func(e *Evidence) { e.Aggregating = 1 },
			"unknown":                 func(e *Evidence) { e.Unknown = 1 },
			"writer outstanding":      func(e *Evidence) { e.Unsettled = 1 },
			"invalid counts":          func(e *Evidence) { e.Future = -1 },
			"disconnected":            func(e *Evidence) { e.InspectionSucceeded = false },
			"unfenced writer":         func(e *Evidence) { e.WritersFenced = false },
			"admission open":          func(e *Evidence) { e.AdmissionClosed = false },
			"consumer still claiming": func(e *Evidence) { e.ClaimsStopped = false },
		} {
			t.Run(string(purpose)+"/"+name, func(t *testing.T) {
				e := completeEvidence()
				change(&e)
				require.ErrorIs(t, validateTransition(testOperation(purpose, Draining), Verifying, e), ErrEvidence)
				require.ErrorIs(t, validateTransition(testOperation(purpose, Verifying), Drained, e), ErrEvidence)
			})
		}
		require.NoError(t, validateTransition(testOperation(purpose, Verifying), Drained, completeEvidence()))
	}
}

func TestQuiescePreservesDurableBacklogButNotUnsettledWrites(t *testing.T) {
	o := testOperation(Quiesce, Verifying)
	e := completeEvidence()
	e.Pending, e.Future, e.Archived, e.Aggregating = 10, 20, 4, 2
	require.NoError(t, validateTransition(o, Quiesced, e))
	e.Unsettled = 1
	require.ErrorIs(t, validateTransition(o, Quiesced, e), ErrEvidence)
	e.Unsettled = 0
	e.Processing = 1
	require.ErrorIs(t, validateTransition(o, Quiesced, e), ErrEvidence)
	require.ErrorIs(t, validateTransition(o, Drained, completeEvidence()), ErrConflict)
}

func TestStaleEvidenceCannotComplete(t *testing.T) {
	for _, purpose := range []Purpose{Quiesce, Drain, DrainPrevious} {
		o := testOperation(purpose, Verifying)
		next := Drained
		if purpose == Quiesce {
			next = Quiesced
		}
		e := completeEvidence()
		e.Epoch--
		require.ErrorIs(t, validateTransition(o, next, e), ErrStale)
		e = completeEvidence()
		e.ConfigurationRevision = "old-config"
		require.ErrorIs(t, validateTransition(o, next, e), ErrStale)
	}
}

func TestStopAndResumeNeverReopenPreviousStore(t *testing.T) {
	for _, state := range []State{Fencing, Draining, Verifying, Stopping, Stopped, Drained} {
		require.ErrorIs(t, validateTransition(testOperation(DrainPrevious, state), Resumed, Evidence{}), ErrConflict)
	}
	require.NoError(t, validateTransition(testOperation(DrainPrevious, Draining), Stopping, Evidence{}))
	require.ErrorIs(t, validateTransition(testOperation(DrainPrevious, Stopping), Stopped, Evidence{}), ErrStale)
	e := completeEvidence()
	e.Pending, e.Future, e.Archived = 2, 1, 1
	require.NoError(t, validateTransition(testOperation(DrainPrevious, Stopping), Stopped, e))
	require.NoError(t, validateTransition(testOperation(DrainPrevious, Stopped), Fencing, Evidence{}))
	for _, purpose := range []Purpose{Quiesce, Drain} {
		require.NoError(t, validateTransition(testOperation(purpose, Fencing), Resuming, Evidence{}))
		require.ErrorIs(t, validateTransition(testOperation(purpose, Fencing), Resumed, Evidence{}), ErrConflict)
		require.ErrorIs(t, validateTransition(testOperation(purpose, Draining), Stopping, Evidence{}), ErrConflict)
	}
}

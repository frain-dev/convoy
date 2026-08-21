package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/pkg/partitions"
)

// TestE2E_IndexRebuild_BootDrainsEveryOwedIndex covers the boot contract: the
// walk rebuilds every index the instance still owes, one at a time, not just
// the names a migration happened to queue.
//
// Only one rebuild may hold the instance slot, so the walk has to chain the
// next owed name when a run finishes. A version that started a fixed pair left
// the rest invalid until the next restart, and a dashboard read reported them
// as still dropped.
func TestE2E_IndexRebuild_BootDrainsEveryOwedIndex(t *testing.T) {
	env := SetupE2E(t)
	ctx := context.Background()

	owed := []struct{ table, name, columns string }{
		{"event_deliveries", "idx_e2e_rebuild_project", "project_id"},
		{"event_deliveries", "idx_e2e_rebuild_status", "status"},
		{"events", "idx_e2e_rebuild_events_project", "project_id"},
	}

	for _, idx := range owed {
		definition := "CREATE INDEX " + idx.name + " ON convoy." + idx.table + " USING btree (" + idx.columns + ")"
		_, err := env.App.DB.GetDB().ExecContext(ctx, definition)
		require.NoError(t, err)

		// Drop it the way an upgrade does after finding it invalid, leaving the
		// rebuild owed.
		_, err = env.App.DB.GetDB().ExecContext(ctx, "DROP INDEX convoy."+idx.name)
		require.NoError(t, err)

		markIndexDropped(t, env, idx.table, idx.name, definition, time.Now().Add(-time.Hour))
		require.False(t, indexExistsAndValid(t, env, idx.name))
	}
	require.Equal(t, len(owed), owedIndexes(t, env))

	svc := partitions.New(env.App.DB, env.App.Logger)
	svc.StartQueuedDroppedIndexes(ctx)

	waitForOwedIndexes(t, env, 0, 3*time.Minute)

	for _, idx := range owed {
		require.True(t, indexExistsAndValid(t, env, idx.name), "%s was never rebuilt", idx.name)
	}

	// Every owed name should have produced its own run, and none may be left
	// running: a slot held after the walk stops blocks the next conversion.
	runs, err := svc.List(ctx, 50)
	require.NoError(t, err)

	rebuilt := make(map[string]partitions.Status)
	for _, run := range runs {
		if run.Operation == partitions.OperationRebuildIndex && run.IndexName != nil {
			rebuilt[*run.IndexName] = run.Status
		}
	}
	for _, idx := range owed {
		require.Equal(t, partitions.StatusCompleted, rebuilt[idx.name], "run for %s", idx.name)
	}
}

// TestE2E_IndexRebuild_SlotHoldsOneRunAtATime confirms a second rebuild is
// refused while one holds the slot, which is what the dashboard's disabled
// Rebuild button reflects.
func TestE2E_IndexRebuild_SlotHoldsOneRunAtATime(t *testing.T) {
	env := SetupE2E(t)
	ctx := context.Background()

	definition := "CREATE INDEX idx_e2e_slot_guard ON convoy.event_deliveries USING btree (created_at)"
	_, err := env.App.DB.GetDB().ExecContext(ctx, definition)
	require.NoError(t, err)
	_, err = env.App.DB.GetDB().ExecContext(ctx, "DROP INDEX convoy.idx_e2e_slot_guard")
	require.NoError(t, err)
	markIndexDropped(t, env, "event_deliveries", "idx_e2e_slot_guard", definition, time.Now().Add(-time.Hour))

	svc := partitions.New(env.App.DB, env.App.Logger)
	run, err := svc.StartIndexRebuild(ctx, "idx_e2e_slot_guard", "e2e")
	require.NoError(t, err)

	// A conversion asks for the same slot and must be told it is taken.
	_, err = svc.Start(ctx, "event_deliveries", partitions.OperationPartition, "e2e")
	require.ErrorIs(t, err, partitions.ErrRunInProgress)

	settled := waitForRunStatus(t, svc, run.UID, 3*time.Minute)
	require.Equal(t, partitions.StatusCompleted, settled.Status)
	require.True(t, indexExistsAndValid(t, env, "idx_e2e_slot_guard"))
	require.Zero(t, owedIndexes(t, env))
}

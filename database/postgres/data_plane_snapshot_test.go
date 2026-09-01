//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/pkg/dataplanestats"
)

func snapshotStore(t *testing.T) *Postgres {
	t.Helper()

	db, _ := getDB(t)
	store, ok := db.(*Postgres)
	require.True(t, ok)

	_, err := store.GetDB().ExecContext(context.Background(), "DELETE FROM convoy.data_plane_snapshots")
	require.NoError(t, err)

	return store
}

func replicaByName(t *testing.T, status dataplanestats.Status, name string) dataplanestats.Replica {
	t.Helper()

	for _, replica := range status.Replicas {
		if replica.Replica == name {
			return replica
		}
	}

	t.Fatalf("no %q replica in %v", name, status.Replicas)
	return dataplanestats.Replica{}
}

// A replica publishes for the life of the process, so the write that matters is
// the second one: it lands on a row the first already wrote. A test that only
// ever inserts into an empty table never exercises the update path an instance
// spends its life in.
func TestPublishSnapshotOverwritesTheSameReplica(t *testing.T) {
	store := snapshotStore(t)
	ctx := context.Background()

	first := dataplanestats.Snapshot{
		Replica:   "pod-1",
		Mode:      "example",
		Running:   true,
		SampledAt: time.Now(),
		Stages:    []dataplanestats.Stage{{Name: "ingest", Queued: 3}},
		Outstanding: []dataplanestats.Backlog{
			{Name: "events_pending", Count: 100, Known: true, AsOf: time.Now()},
		},
	}
	require.NoError(t, store.PublishSnapshot(ctx, first))

	second := first
	second.SampledAt = time.Now()
	second.Stages = []dataplanestats.Stage{{Name: "ingest", Queued: 9}}
	second.Outstanding = []dataplanestats.Backlog{
		{Name: "events_pending", Count: 84000, Known: true, AsOf: time.Now()},
	}
	require.NoError(t, store.PublishSnapshot(ctx, second))

	status, err := store.DataPlaneStatus(ctx, time.Minute)
	require.NoError(t, err)
	require.Len(t, status.Replicas, 1)

	replica := replicaByName(t, status, "pod-1")
	require.Equal(t, 9, replica.Stages[0].Queued)
	require.Equal(t, int64(84000), replica.Outstanding[0].Count)
	require.True(t, replica.Outstanding[0].Known)
	require.False(t, replica.Stale)
}

// Two replicas are two rows. A replica may only ever overwrite itself, or a
// scaled-out deployment reports one replica's depth as the whole fleet's.
func TestPublishSnapshotKeepsReplicasApart(t *testing.T) {
	store := snapshotStore(t)
	ctx := context.Background()

	for _, name := range []string{"pod-1", "pod-2"} {
		require.NoError(t, store.PublishSnapshot(ctx, dataplanestats.Snapshot{
			Replica:   name,
			Mode:      "example",
			Running:   true,
			SampledAt: time.Now(),
		}))
	}

	status, err := store.DataPlaneStatus(ctx, time.Minute)
	require.NoError(t, err)
	require.Len(t, status.Replicas, 2)
}

// A replica that stopped publishing is stale, not current. Its numbers describe
// a moment that has passed, and the threshold the server applied travels with
// the answer so no client has to guess it.
func TestDataPlaneStatusMarksReplicasStale(t *testing.T) {
	store := snapshotStore(t)
	ctx := context.Background()

	require.NoError(t, store.PublishSnapshot(ctx, dataplanestats.Snapshot{
		Replica:   "pod-old",
		Mode:      "example",
		Running:   true,
		SampledAt: time.Now().Add(-2 * time.Minute),
	}))
	require.NoError(t, store.PublishSnapshot(ctx, dataplanestats.Snapshot{
		Replica:   "pod-new",
		Mode:      "example",
		Running:   true,
		SampledAt: time.Now(),
	}))

	status, err := store.DataPlaneStatus(ctx, 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(30), status.StaleAfterSeconds)
	require.True(t, replicaByName(t, status, "pod-old").Stale)
	require.False(t, replicaByName(t, status, "pod-new").Stale)
	require.Greater(t, replicaByName(t, status, "pod-old").AgeSeconds, float64(100))
}

// A replica that was scaled away has to leave the page rather than keep claiming
// depth it no longer holds, and expiry must not take the live one with it.
func TestExpireSnapshotsRemovesOnlyDeadReplicas(t *testing.T) {
	store := snapshotStore(t)
	ctx := context.Background()

	require.NoError(t, store.PublishSnapshot(ctx, dataplanestats.Snapshot{
		Replica:   "pod-gone",
		Mode:      "example",
		SampledAt: time.Now().Add(-time.Hour),
	}))
	require.NoError(t, store.PublishSnapshot(ctx, dataplanestats.Snapshot{
		Replica:   "pod-live",
		Mode:      "example",
		Running:   true,
		SampledAt: time.Now(),
	}))

	removed, err := store.ExpireSnapshots(ctx, dataplanestats.ExpireAfter)
	require.NoError(t, err)
	require.Equal(t, int64(1), removed)

	status, err := store.DataPlaneStatus(ctx, time.Minute)
	require.NoError(t, err)
	require.Len(t, status.Replicas, 1)
	require.Equal(t, "pod-live", status.Replicas[0].Replica)

	// Running twice is the steady state: a second pass finds nothing left to do
	// and must not disturb the replica that is still publishing.
	removed, err = store.ExpireSnapshots(ctx, dataplanestats.ExpireAfter)
	require.NoError(t, err)
	require.Zero(t, removed)

	status, err = store.DataPlaneStatus(ctx, time.Minute)
	require.NoError(t, err)
	require.Len(t, status.Replicas, 1)
}

// A row is keyed on the replica, so publishing without one would silently
// collapse the fleet onto a single row.
func TestPublishSnapshotRejectsAnUnnamedReplica(t *testing.T) {
	store := snapshotStore(t)

	require.Error(t, store.PublishSnapshot(context.Background(), dataplanestats.Snapshot{Mode: "example"}))
}

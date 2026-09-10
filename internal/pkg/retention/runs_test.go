package retention

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRowCountContextDetachesFromJobCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	countCtx, stop := rowCountContext(ctx)
	defer stop()

	parentDL, ok := ctx.Deadline()
	require.True(t, ok)
	countDL, ok := countCtx.Deadline()
	require.True(t, ok)
	require.True(t, countDL.Before(parentDL), "count deadline must be earlier than the job so a slow COUNT cannot consume Maintain")

	cancel()
	require.NoError(t, countCtx.Err(), "count ctx inherited job cancellation")
}

func TestRowCountContextSkipsWhenJobHasNoBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	countCtx, stop := rowCountContext(ctx)
	defer stop()
	require.Error(t, countCtx.Err(), "counts must skip when remaining deadline is at or under the cap")
}

func TestRowCountContextSkipsWhenJobAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	countCtx, stop := rowCountContext(ctx)
	defer stop()
	require.Error(t, countCtx.Err())
}

func TestCountExpiredPartitionRowCountsSkipsCancelledJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// A cancelled job must not open a query. Passing a nil db would panic if
	// the scan still used the job context.
	require.Nil(t, countExpiredPartitionRowCounts(ctx, nil))
}

func TestRunDetailsValue(t *testing.T) {
	d := RunDetails{{
		Table:             "events",
		DroppedPartitions: []string{"convoy.events_p20260901"},
		DroppedRows:       100,
	}}
	v, err := d.Value()
	require.NoError(t, err)
	b, ok := v.([]byte)
	require.True(t, ok)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(b, &got))
	require.Len(t, got, 1)
	require.Equal(t, "events", got[0]["table"])
	require.Equal(t, []any{"convoy.events_p20260901"}, got[0]["dropped_partitions"])
	require.EqualValues(t, 100, got[0]["dropped_rows"])

	empty, err := RunDetails(nil).Value()
	require.NoError(t, err)
	emptyBytes, ok := empty.([]byte)
	require.True(t, ok)
	require.Equal(t, "[]", string(emptyBytes))
}

// after must mirror snapshotManagedPartitions: gopartman keeps dropped
// children in partman.partitions with status='dropped', so the after
// snapshot excludes them or diff would never see a Maintain drop.
func TestDiffPartitionDrops(t *testing.T) {
	before := partitionSnapshot{
		"events": {
			"convoy.events_p20260901": {},
			"convoy.events_p20260902": {},
		},
		"event_deliveries": {
			"convoy.event_deliveries_p20260901": {},
		},
	}
	after := partitionSnapshot{
		"events": {
			"convoy.events_p20260902": {},
		},
		"event_deliveries": {
			"convoy.event_deliveries_p20260901": {},
		},
	}

	beforeCounts := map[string]map[string]int64{
		"events": {
			"convoy.events_p20260901": 100,
			"convoy.events_p20260902": 0,
		},
		"event_deliveries": {
			"convoy.event_deliveries_p20260901": 0,
		},
	}

	details := diffPartitionDrops(before, after, beforeCounts)
	require.Len(t, details, len(RetentionTables))

	require.Equal(t, "events", details[0].Table)
	require.Equal(t, []string{"convoy.events_p20260901"}, details[0].DroppedPartitions)
	require.Equal(t, int64(100), details[0].DroppedRows)
	require.Equal(t, int64(100), details[0].DroppedPartitionRows["convoy.events_p20260901"])

	require.Equal(t, "events_search", details[1].Table)
	require.Empty(t, details[1].DroppedPartitions)

	require.Equal(t, "event_deliveries", details[2].Table)
	require.Empty(t, details[2].DroppedPartitions)
}

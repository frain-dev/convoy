package batch_tracker

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	dbpostgres "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/testenv"
)

var testInfra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background(), testenv.WithoutRedis())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}
	testInfra = res
	code := m.Run()
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
	}
	os.Exit(code)
}

func setupTracker(t *testing.T) Tracker {
	t.Helper()
	conn, err := testInfra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	return NewPostgresTracker(dbpostgres.NewFromConnection(conn).GetDB())
}

// TestPostgresTrackerRoundTrip reads every column back through sqlx. Writes were
// covered by the handler paths but no test ever scanned a row, which is how the
// struct came to carry json tags and no db tags: sqlx lowercases the field name
// by default, so batch_id had no destination and the scan failed.
func TestPostgresTrackerRoundTrip(t *testing.T) {
	tr := setupTracker(t)
	ctx := context.Background()
	id := ulid.Make().String()

	require.NoError(t, tr.CreateBatch(ctx, id, 10, "Retry", "1h", "evt-1"))
	require.NoError(t, tr.IncrementProcessed(ctx, id, 4))
	require.NoError(t, tr.IncrementFailed(ctx, id, 1))

	got, err := tr.GetBatch(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, got.BatchID)
	require.Equal(t, BatchStatusRunning, got.Status)
	require.Equal(t, int64(10), got.TotalCount)
	require.Equal(t, int64(4), got.ProcessedCount)
	require.Equal(t, int64(1), got.FailedCount)
	require.Equal(t, "Retry", got.StatusFilter)
	require.Equal(t, "1h", got.TimePeriod)
	require.Equal(t, "evt-1", got.EventID)
	require.False(t, got.StartTime.IsZero())
	require.Nil(t, got.EndTime)

	require.NoError(t, tr.CompleteBatch(ctx, id))
	done, err := tr.GetBatch(ctx, id)
	require.NoError(t, err)
	require.Equal(t, BatchStatusCompleted, done.Status)
	require.NotNil(t, done.EndTime)

	listed, err := tr.ListBatches(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, id, listed[0].BatchID)

	require.NoError(t, tr.DeleteBatch(ctx, id))
	_, err = tr.GetBatch(ctx, id)
	require.ErrorContains(t, err, "batch not found")
}

// TestPostgresTrackerFailBatch covers the error column, which the round trip
// leaves empty.
func TestPostgresTrackerFailBatch(t *testing.T) {
	tr := setupTracker(t)
	ctx := context.Background()
	id := ulid.Make().String()

	require.NoError(t, tr.CreateBatch(ctx, id, 1, "Scheduled", "5h", ""))
	require.NoError(t, tr.FailBatch(ctx, id, "boom"))

	got, err := tr.GetBatch(ctx, id)
	require.NoError(t, err)
	require.Equal(t, BatchStatusFailed, got.Status)
	require.Equal(t, "boom", got.Error)
	require.NotNil(t, got.EndTime)
}

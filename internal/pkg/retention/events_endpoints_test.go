package retention

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
)

// convoy.events_endpoints has carried no foreign key since sql/1724932863.sql
// rebuilt it with LIKE ... INCLUDING CONSTRAINTS, so nothing removed a row when
// retention dropped the event it points at. On a partitioned instance that had
// been running for months the table reached 4 GB of rows whose events were long
// gone.
func TestSweepRemovesEventEndpointsWhoseEventIsGone(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)
	seedEventEndpoint(t, db, ulid.Make().String(), endpointID)

	newDropPolicy(t, db, 24*time.Hour).sweepOrphanedEventEndpoints(ctx)

	require.Zero(t, countEventEndpoints(t, ctx, db),
		"a row whose event retention already dropped survived the sweep")
}

func TestSweepKeepsEventEndpointsWhoseEventSurvives(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)
	seedEventEndpoint(t, db, liveEventID(t, ctx, db, projectID), endpointID)

	newDropPolicy(t, db, 24*time.Hour).sweepOrphanedEventEndpoints(ctx)

	require.Equal(t, 1, countEventEndpoints(t, ctx, db),
		"the sweep deleted a row whose event is still in convoy.events")
}

// An event is written once and fans out to every endpoint it matched, so the
// unit the sweep has to get right is the event, not the row: every row for a
// dropped event goes, and every row for a live one stays, even when both events
// point at the same endpoints.
func TestSweepRemovesAnExpiredEventsWholeFanOutAndKeepsALiveOnes(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	live := liveEventID(t, ctx, db, projectID)
	expired := ulid.Make().String()

	endpoints := []string{
		seedEndpoint(t, db, projectID),
		seedEndpoint(t, db, projectID),
		seedEndpoint(t, db, projectID),
	}
	for _, endpointID := range endpoints {
		seedEventEndpoint(t, db, live, endpointID)
		seedEventEndpoint(t, db, expired, endpointID)
	}

	newDropPolicy(t, db, 24*time.Hour).sweepOrphanedEventEndpoints(ctx)

	require.Equal(t, len(endpoints), countEventEndpointsFor(t, ctx, db, live),
		"the sweep lost part of a live event's fan-out")
	require.Zero(t, countEventEndpointsFor(t, ctx, db, expired),
		"the sweep left part of an expired event's fan-out behind")
}

// The search path reads convoy.events_endpoints joined to convoy.events_search,
// not to convoy.events, so an event that is still searchable must keep its
// endpoint rows even after the convoy.events side has expired.
func TestSweepKeepsEventEndpointsWhoseEventSurvivesOnlyInSearch(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)

	searchOnly := ulid.Make().String()
	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.events_search (id, event_type, project_id, raw, data, url_path)
        VALUES ($1, 'test.event', $2, '{}', '{}'::bytea, '/')`, searchOnly, projectID)
	require.NoError(t, err)
	seedEventEndpoint(t, db, searchOnly, endpointID)

	newDropPolicy(t, db, 24*time.Hour).sweepOrphanedEventEndpoints(ctx)

	require.Equal(t, 1, countEventEndpoints(t, ctx, db),
		"the sweep deleted a row the search path still reads")
}

// One statement must not try to delete a backlog in a single pass, which is the
// failure being fixed rather than an implementation of the fix.
func TestOneSweepBatchRemovesAtMostTheBatchBound(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)
	const excess = 200
	seedOrphanedEventEndpoints(t, ctx, db, endpointID, orphanBatchRows+excess)

	policy := newDropPolicy(t, db, 24*time.Hour)

	deleted, err := policy.deleteOrphanedEventEndpoints(ctx)
	require.NoError(t, err)
	require.EqualValues(t, orphanBatchRows, deleted, "one batch was not bounded by orphanBatchRows")
	require.Equal(t, excess, countEventEndpoints(t, ctx, db))

	// The backlog is cleared by repetition, so the same call run to exhaustion
	// finishes the table off.
	policy.sweepOrphanedEventEndpoints(ctx)
	require.Zero(t, countEventEndpoints(t, ctx, db))
}

// The read bounds itself by rows, so a batch's limit can fall inside an event's
// fan-out. The delete names events rather than rows, so it takes the rest of
// that fan-out with it instead of leaving a part of one behind.
func TestOneSweepBatchNeverLeavesPartOfAFanOut(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	const fanOut = 3
	// More events than one batch can name, so a remainder is left to inspect.
	const events = orphanBatchRows + 500
	for range fanOut {
		seedOrphanedEventEndpoints(t, ctx, db, seedEndpoint(t, db, projectID), events)
	}

	deleted, err := newDropPolicy(t, db, 24*time.Hour).deleteOrphanedEventEndpoints(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(orphanBatchRows),
		"the batch stopped at the row limit and left the rest of a fan-out behind")
	require.Empty(t, partialFanOuts(t, ctx, db, fanOut),
		"the batch left events holding some of their rows and not others")
}

func TestSweepIsANoopOnACleanTable(t *testing.T) {
	db, ctx := setupTestDB(t)

	policy := newDropPolicy(t, db, 24*time.Hour)
	policy.sweepOrphanedEventEndpoints(ctx)

	deleted, err := policy.deleteOrphanedEventEndpoints(ctx)
	require.NoError(t, err)
	require.Zero(t, deleted)
	require.Zero(t, countEventEndpoints(t, ctx, db))
}

// Detaching renames the partitioned parent to convoy.events_partitioned and
// only copies the rows back under the live name on a later statement. Reading
// convoy.events during that window reports live events as missing, so the sweep
// has to stand down until the leftover name is gone.
// Retention and conversion use different locks, so a conversion can start after
// the sweep's first settlement check. Re-checking before each batch keeps later
// iterations from deleting rows whose events still live on leftover relations.
func TestSweepStopsWhenSettlementBecomesFalseMidRun(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)
	const excess = 200
	seedOrphanedEventEndpoints(t, ctx, db, endpointID, orphanBatchRows+excess)

	afterOrphanSweepBatchHook = func() {
		_, err := db.GetDB().ExecContext(ctx,
			`CREATE TABLE convoy.events_partitioned (LIKE convoy.events)`)
		require.NoError(t, err)
	}
	t.Cleanup(func() { afterOrphanSweepBatchHook = nil })

	newDropPolicy(t, db, 24*time.Hour).sweepOrphanedEventEndpoints(ctx)

	require.Equal(t, excess, countEventEndpoints(t, ctx, db),
		"the sweep kept deleting after event tables became unsettled")
}

func TestSweepSkipsWhileADetachHasNotDrained(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)
	seedEventEndpoint(t, db, ulid.Make().String(), endpointID)

	_, err := db.GetDB().ExecContext(ctx,
		`CREATE TABLE convoy.events_partitioned (LIKE convoy.events)`)
	require.NoError(t, err)

	newDropPolicy(t, db, 24*time.Hour).sweepOrphanedEventEndpoints(ctx)

	require.Equal(t, 1, countEventEndpoints(t, ctx, db),
		"the sweep ran mid-detach, when absence from convoy.events is not a verdict")
}

// The instance this was found on had convoy.events partitioned, which is the
// only shape retention runs on, and it is also the shape that makes the lookup
// fan out across every child.
func TestSweepRemovesOrphansOnAPartitionedEventsTable(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)
	live := liveEventID(t, ctx, db, projectID)
	seedEventEndpoint(t, db, live, endpointID)
	seedEventEndpoint(t, db, ulid.Make().String(), endpointID)

	partitionTable(t, ctx, db, "events")
	partitionTable(t, ctx, db, "events_search")

	newDropPolicy(t, db, 24*time.Hour).sweepOrphanedEventEndpoints(ctx)

	require.Equal(t, 1, countEventEndpoints(t, ctx, db))
	require.Equal(t, 1, countEventEndpointsFor(t, ctx, db, live))
}

// Running out of budget is how a run with a backlog is meant to end, so it has
// to stay distinguishable from a fault after the driver has wrapped it, or
// every night of a long cleanup reports an error it should not.
func TestSweepReportsACancelledRunAsTheClock(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	seedEventEndpoint(t, db, ulid.Make().String(), seedEndpoint(t, db, projectID))

	spent, cancel := context.WithCancel(ctx)
	cancel()

	_, err := newDropPolicy(t, db, 24*time.Hour).deleteOrphanedEventEndpoints(spent)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, countEventEndpoints(t, ctx, db))
}

// The sweep is only worth anything if the nightly job runs it, and it runs
// after the partition maintenance that creates the orphans in the first place.
func TestPerformSweepsOrphanedEventEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)
	endpointID := seedEndpoint(t, db, projectID)
	live := liveEventID(t, ctx, db, projectID)
	seedEventEndpoint(t, db, live, endpointID)
	seedEventEndpoint(t, db, ulid.Make().String(), endpointID)

	partitionAll(t, ctx, db)

	require.NoError(t, newDropPolicy(t, db, 24*time.Hour).Perform(ctx))

	require.Equal(t, 1, countEventEndpoints(t, ctx, db))
	require.Equal(t, 1, countEventEndpointsFor(t, ctx, db, live))
}

func liveEventID(t *testing.T, ctx context.Context, db database.Database, projectID string) string {
	t.Helper()

	var id string
	require.NoError(t, db.GetDB().QueryRowContext(ctx,
		`SELECT id FROM convoy.events WHERE project_id = $1`, projectID).Scan(&id))
	return id
}

func seedEndpoint(t *testing.T, db database.Database, projectID string) string {
	t.Helper()

	endpointID := ulid.Make().String()
	_, err := db.GetDB().ExecContext(context.Background(), `
        INSERT INTO convoy.endpoints (
            id, name, status, url, http_timeout, rate_limit, rate_limit_duration,
            advanced_signatures, project_id, secrets)
        VALUES ($1, $2, 'active', 'https://example.com', 10000, 1000, 60, false, $3, '[]'::jsonb)`,
		endpointID, "retention-"+endpointID, projectID)
	require.NoError(t, err)

	return endpointID
}

func seedEventEndpoint(t *testing.T, db database.Database, eventID, endpointID string) {
	t.Helper()

	_, err := db.GetDB().ExecContext(context.Background(),
		`INSERT INTO convoy.events_endpoints (event_id, endpoint_id) VALUES ($1, $2)`, eventID, endpointID)
	require.NoError(t, err)
}

// seedOrphanedEventEndpoints writes count rows naming events that were never
// inserted, which is the state retention leaves behind once it drops the
// partition an event lived in.
func seedOrphanedEventEndpoints(t *testing.T, ctx context.Context, db database.Database, endpointID string, count int) {
	t.Helper()

	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.events_endpoints (event_id, endpoint_id)
        SELECT 'expired' || LPAD(i::TEXT, 19, '0'), $1
        FROM generate_series(1, $2) AS i`, endpointID, count)
	require.NoError(t, err)
}

// partialFanOuts names the events left holding a number of rows other than the
// fan-out they were written with.
func partialFanOuts(t *testing.T, ctx context.Context, db database.Database, fanOut int) []string {
	t.Helper()

	rows, err := db.GetDB().QueryContext(ctx, `
        SELECT event_id FROM convoy.events_endpoints
        GROUP BY event_id HAVING count(*) <> $1`, fanOut)
	require.NoError(t, err)
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())

	return ids
}

func countEventEndpoints(t *testing.T, ctx context.Context, db database.Database) int {
	t.Helper()

	var count int
	require.NoError(t, db.GetDB().QueryRowContext(ctx,
		`SELECT count(*) FROM convoy.events_endpoints`).Scan(&count))
	return count
}

func countEventEndpointsFor(t *testing.T, ctx context.Context, db database.Database, eventID string) int {
	t.Helper()

	var count int
	require.NoError(t, db.GetDB().QueryRowContext(ctx,
		`SELECT count(*) FROM convoy.events_endpoints WHERE event_id = $1`, eventID).Scan(&count))
	return count
}

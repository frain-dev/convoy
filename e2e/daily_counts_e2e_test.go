package e2e

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/frain-dev/convoy/database/postgres"
)

// TestE2E_DailyCounts_SummaryMatchesLiveAcrossRepeatedRefreshes drives the
// dashboard summary the way a real instance does: deliver events through the
// API, then let the every-minute rollup job run more than once.
//
// The job is scheduled at "* * * * *", so every run after the first rewrites
// days the rollup already holds. A single refresh against an empty table is a
// state a real instance leaves after sixty seconds and never returns to, which
// is how a rollup that could not be rewritten still read correctly under test
// while failing on every run in production.
//
// The number the rollup has to reproduce comes from the live table, through the
// same wait that settles the deliveries, and not from an early read of the
// summary endpoint. That endpoint switches to the rollup the moment the backfill
// is marked complete, and on a database holding no deliveries the backfill
// completes on its first run with nothing to walk, so whether an early read is
// live or rollup depends only on where the scheduler's minute boundary fell
// during setup. Live and rollup agreement per status is asserted in
// TestStatusTotalsAgreePerStatusAcrossSources, where nothing competes.
func TestE2E_DailyCounts_SummaryMatchesLiveAcrossRepeatedRefreshes(t *testing.T) {
	env := SetupE2E(t)
	ctx := context.Background()

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	port := 19911
	StartMockWebhookServer(t, manifest, done, &counter, port)

	client := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	ownerID := "daily-counts-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, client, port, ownerID)
	CreateSubscriptionViaSDK(t, client, endpoint.UID, []string{"*"})

	const eventType = "daily.counts"
	const sent = 3
	for i := 0; i < sent; i++ {
		SendEventViaSDK(t, client, endpoint.UID, eventType, "trace-"+ulid.Make().String())
	}
	waitForDeliveryCount(t, env, env.Project.UID, sent, 60*time.Second)

	link := seedPortalLink(t, env, ownerID)
	start := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02T15:04:05")
	end := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02T15:04:05")

	// Three runs of the real job: the first writes today, the rest rewrite it.
	job := dailyCountsJob(env)
	for i := 0; i < 3; i++ {
		runWorkerJob(t, job)
	}

	// The summary only reads the rollup once the backfill is marked complete, so
	// without this the assertions below could pass on a live scan and say nothing
	// about the rollup.
	var completed bool
	require.NoError(t, env.App.DB.GetDB().QueryRowContext(ctx, `
		SELECT completed_at IS NOT NULL
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = 'backfill'`).Scan(&completed))
	require.True(t, completed, "summary is still answering from a live scan")

	fromRollup := fetchPortalSummary(t, env, link, "daily", start, end)
	require.Equal(t, uint64(sent), fromRollup.EventsSent, "rollup disagrees with the deliveries the project holds")

	// Summed rather than pinned to one bucket: a run that crosses UTC midnight
	// splits these deliveries across two days.
	var bucketed uint64
	for _, in := range nonZeroBuckets(fromRollup) {
		bucketed += in.Count
	}
	require.Equal(t, uint64(sent), bucketed, "chart buckets disagree with the summary total")

	// A delivery that lands after the rollup was first written must appear on
	// the next refresh, which only holds if a populated day can be rewritten.
	SendEventViaSDK(t, client, endpoint.UID, eventType, "trace-late")
	waitForDeliveryCount(t, env, env.Project.UID, sent+1, 60*time.Second)
	runWorkerJob(t, job)

	updated := fetchPortalSummary(t, env, link, "daily", start, end)
	require.Equal(t, uint64(sent+1), updated.EventsSent, "rollup did not pick up a later delivery")
}

// TestE2E_DailyCounts_BackfillCompletesAndPrunes walks the backfill to
// completion through the job, then checks the rollup holds no day the live
// table no longer covers.
func TestE2E_DailyCounts_BackfillCompletesAndPrunes(t *testing.T) {
	env := SetupE2E(t)
	ctx := context.Background()
	db := env.App.DB.(*postgres.Postgres)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	port := 19912
	StartMockWebhookServer(t, manifest, done, &counter, port)

	client := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	ownerID := "backfill-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, client, port, ownerID)
	CreateSubscriptionViaSDK(t, client, endpoint.UID, []string{"*"})

	const eventType = "backfill.counts"
	SendEventViaSDK(t, client, endpoint.UID, eventType, "trace-"+ulid.Make().String())
	waitForDeliveryCount(t, env, env.Project.UID, 1, 60*time.Second)

	// Age the delivery so the backfill has history to walk.
	_, err := db.GetDB().ExecContext(ctx, `
		UPDATE convoy.event_deliveries
		SET created_at = NOW() - INTERVAL '3 days', updated_at = NOW() - INTERVAL '3 days'
		WHERE project_id = $1`, env.Project.UID)
	require.NoError(t, err)

	// The job advances one day per run, so run it enough times to finish and
	// then a few more. Every run past completion re-refreshes populated days.
	job := dailyCountsJob(env)
	for i := 0; i < 8; i++ {
		runWorkerJob(t, job)
	}

	var completed bool
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT completed_at IS NOT NULL
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = 'backfill'`).Scan(&completed))
	require.True(t, completed, "backfill never completed")

	var stale int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM convoy.event_delivery_daily_counts
		WHERE day < (
			SELECT MIN((created_at AT TIME ZONE 'UTC')::date)
			FROM convoy.event_deliveries
			WHERE deleted_at IS NULL
		)`).Scan(&stale))
	require.Zero(t, stale, "rollup holds days the live table no longer covers")
}

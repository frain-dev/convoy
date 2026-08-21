package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
)

// TestE2E_Metrics_ServedFromSnapshotGenerations scrapes /metrics off the
// running server the way Prometheus does.
//
// The gauges read a snapshot generation that a scheduled job writes, not a
// GROUP BY run inside the scrape: a scrape that queried event_deliveries
// directly took the whole table with it on a large instance. So the contract is
// that a scrape answers from the last generation, answers before any generation
// exists, and picks up a newer generation once the collector refreshes.
//
// The server runs without a worker here. Prometheus' registry is a process
// global while the collectors are per-instance, so a server and an agent in one
// process register the same collector twice. Real deployments run them as
// separate processes; the job is invoked directly instead.
func TestE2E_Metrics_ServedFromSnapshotGenerations(t *testing.T) {
	metrics.Reset()
	t.Cleanup(metrics.Reset)

	env := SetupE2EWithoutWorker(t, func(cfg *config.Configuration) {
		cfg.Metrics.IsEnabled = true
		cfg.Metrics.Backend = config.PrometheusMetricsProvider
		// Collect serves the cached snapshot and refreshes in the background,
		// gated by SampleTime. The default 5s would make the poll below wait on
		// a window that has nothing to do with the contract under test.
		cfg.Metrics.Prometheus.SampleTime = 1
	})
	ctx := context.Background()

	// A scrape before any snapshot exists must serve, not error, and must not
	// run the GROUP BY itself.
	require.NotContains(t, scrapeMetrics(t, env), "convoy_event_queue_total",
		"a gauge appeared before any snapshot was written")

	// The event queue snapshot is a GROUP BY over convoy.events, so the table
	// has to hold rows for the generation to carry any sample at all.
	endpoint, err := testdb.SeedEndpoint(env.App.DB, env.Project, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = testdb.SeedEvent(env.App.DB, endpoint, env.Project.UID, ulid.Make().String(),
			"metrics.sample", "", []byte(`{}`))
		require.NoError(t, err)
	}

	runWorkerJob(t, queueMetricsJob(env))
	first := snapshotGeneration(t, env)
	require.NotZero(t, first, "snapshot job wrote no generation for the scrape to read")

	// The first scrape after the job still serves the empty cache and only
	// schedules the refresh, so the gauge arrives on a later scrape.
	body := scrapeUntil(t, env, "convoy_event_queue_total", 30*time.Second)
	require.Contains(t, body, `convoy_event_queue_total{project="`+env.Project.UID+`"`,
		"gauge carries no series for the project that owns the events")

	// Later runs advance the generation and must retire the previous one rather
	// than leave both live, which would double every gauge.
	for i := 0; i < 3; i++ {
		runWorkerJob(t, queueMetricsJob(env))
	}
	require.Greater(t, snapshotGeneration(t, env), first, "generation did not advance")

	var liveGenerations int
	require.NoError(t, env.App.DB.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT generation) FROM convoy.metrics_event_queue`).Scan(&liveGenerations))
	require.LessOrEqual(t, liveGenerations, 1, "stale snapshot generations were never retired")

	// One series per project, not one per retired generation.
	final := scrapeUntil(t, env, "convoy_event_queue_total", 30*time.Second)
	require.Equal(t, 1, strings.Count(final, "convoy_event_queue_total{"),
		"a scrape emitted more than one series for a single project")
}

// scrapeUntil polls /metrics until a family appears. The collector answers from
// a cached snapshot and refreshes in the background, so the generation a job
// just wrote reaches the wire on a later scrape, never the next one.
func scrapeUntil(t *testing.T, env *E2ETestEnv, family string, timeout time.Duration) string {
	t.Helper()

	var body string
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body = scrapeMetrics(t, env)
		if strings.Contains(body, family) {
			return body
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.Contains(t, body, family, "%s never reached a scrape within %v", family, timeout)
	return body
}

// snapshotGeneration reads the generation the scrape would join against.
func snapshotGeneration(t *testing.T, env *E2ETestEnv) int64 {
	t.Helper()

	var generation int64
	err := env.App.DB.GetDB().QueryRowContext(context.Background(), `
		SELECT COALESCE(MAX(generation), 0) FROM convoy.metrics_snapshot_meta`).Scan(&generation)
	require.NoError(t, err)
	return generation
}

// TestE2E_Metrics_ClosedUnlessEnabled confirms a default install exposes
// nothing on /metrics.
func TestE2E_Metrics_ClosedUnlessEnabled(t *testing.T) {
	env := SetupE2E(t)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(env.ServerURL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

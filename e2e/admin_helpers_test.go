package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/partitions"
	"github.com/frain-dev/convoy/worker/task"
)

// serverResponse mirrors util.ServerResponse enough to unwrap the data object.
type serverResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// authGet reads a path off the running server with a bearer credential and
// returns the unwrapped data payload. Tests assert on what a client sees, not
// on what a service returned in process.
func authGet(t *testing.T, env *E2ETestEnv, path, token string) json.RawMessage {
	t.Helper()

	body, status := rawAuthGet(t, env, path, token)
	require.Equal(t, http.StatusOK, status, string(body))

	var wrapped serverResponse
	require.NoError(t, json.Unmarshal(body, &wrapped))
	return wrapped.Data
}

// rawAuthGet returns the body and status without asserting either, for tests
// that expect a rejection.
func rawAuthGet(t *testing.T, env *E2ETestEnv, path, token string) ([]byte, int) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, env.ServerURL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(api.VersionHeader, config.DefaultAPIVersion)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body, resp.StatusCode
}

// seedPortalLink mints a portal link for an owner. The portal token is the only
// credential that reads the dashboard summary over HTTP without a dashboard
// login, so it is how these tests exercise the read path.
func seedPortalLink(t *testing.T, env *E2ETestEnv, ownerID string) *datastore.PortalLink {
	t.Helper()

	link, err := testdb.SeedPortalLink(env.App.DB, env.Project, ownerID)
	require.NoError(t, err)
	return link
}

// fetchPortalSummary reads the dashboard summary the way the portal does.
//
// The handler caches each answer for an hour and serves the cached copy while
// refreshing in the background, so a test that read twice would compare a
// stale value against itself and pass no matter what the rollup holds. Dropping
// the key the handler builds forces a real computation on every call.
func fetchPortalSummary(t *testing.T, env *E2ETestEnv, link *datastore.PortalLink, period, startDate, endDate string) models.DashboardSummary {
	t.Helper()

	const format = "2006-01-02T15:04:05"
	startT, err := time.Parse(format, startDate)
	require.NoError(t, err)
	endT, err := time.Parse(format, endDate)
	require.NoError(t, err)

	key := fmt.Sprintf("%v:%v:%v:%v:%v", env.Project.UID, startT.Unix(), endT.Unix(), period, link.UID)
	require.NoError(t, env.App.Cache.Delete(context.Background(), key))

	path := fmt.Sprintf("/portal-api/dashboard/summary?startDate=%s&endDate=%s&type=%s", startDate, endDate, period)
	data := authGet(t, env, path, link.Token)

	var summary models.DashboardSummary
	require.NoError(t, json.Unmarshal(data, &summary))
	return summary
}

// waitForDeliveryCount polls until the project holds the expected number of
// deliveries. The rollup aggregates deliveries, so this is the state that has
// to settle before a summary read means anything.
func waitForDeliveryCount(t *testing.T, env *E2ETestEnv, projectID string, want int, timeout time.Duration) {
	t.Helper()

	var got int
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		require.NoError(t, env.App.DB.GetDB().QueryRowContext(context.Background(), `
			SELECT COUNT(*) FROM convoy.event_deliveries
			WHERE project_id = $1 AND deleted_at IS NULL`, projectID).Scan(&got))
		if got >= want {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.Equal(t, want, got, "deliveries did not settle")
}

// nonZeroBuckets drops the empty buckets the summary pads a period with, so a
// live answer and a rollup answer can be compared on the days that carry data.
func nonZeroBuckets(summary models.DashboardSummary) []datastore.EventInterval {
	if summary.PeriodData == nil {
		return nil
	}
	var out []datastore.EventInterval
	for _, in := range *summary.PeriodData {
		if in.Count > 0 {
			out = append(out, in)
		}
	}
	return out
}

// runWorkerJob runs a scheduled task through its real handler closure, so the
// job's lock and service wiring are exercised rather than bypassed. Callers run
// it more than once on purpose: the second run is the one that operates on
// state the first run wrote, which is every run after a process's first minute.
func runWorkerJob(t *testing.T, handler func(context.Context, *asynq.Task) error) {
	t.Helper()
	require.NoError(t, handler(context.Background(), asynq.NewTask("scheduled", nil)))
}

func dailyCountsJob(env *E2ETestEnv) func(context.Context, *asynq.Task) error {
	return task.RefreshEventDeliveryDailyCounts(env.App.Logger, env.App.DB, env.App.Broker.JobLocker)
}

func queueMetricsJob(env *E2ETestEnv) func(context.Context, *asynq.Task) error {
	return task.RefreshQueueMetricsSnapshot(env.App.Logger, env.App.DB, env.App.Broker.JobLocker)
}

// markIndexDropped records an owed rebuild the way an upgrade does when it
// drops an index it found invalid.
func markIndexDropped(t *testing.T, env *E2ETestEnv, table, name, definition string, droppedAt time.Time) {
	t.Helper()

	_, err := env.App.DB.GetDB().ExecContext(context.Background(), `
		INSERT INTO convoy.dropped_indexes (index_name, table_name, definition, dropped_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (index_name) DO UPDATE
		SET table_name = EXCLUDED.table_name,
		    definition = EXCLUDED.definition,
		    dropped_at = EXCLUDED.dropped_at,
		    rebuilt_at = NULL`, name, table, definition, droppedAt)
	require.NoError(t, err)
}

// owedIndexes counts rebuilds still outstanding.
func owedIndexes(t *testing.T, env *E2ETestEnv) int {
	t.Helper()

	var count int
	require.NoError(t, env.App.DB.GetDB().QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM convoy.dropped_indexes WHERE rebuilt_at IS NULL`).Scan(&count))
	return count
}

// waitForOwedIndexes polls until every owed rebuild has been claimed, which is
// the boot contract: one name at a time, but all of them before the walk stops.
func waitForOwedIndexes(t *testing.T, env *E2ETestEnv, want int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if got := owedIndexes(t, env); got == want {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	require.Equal(t, want, owedIndexes(t, env), "owed rebuilds did not drain")
}

// waitForRunStatus polls a partition or rebuild run until it leaves the
// in-flight states.
func waitForRunStatus(t *testing.T, svc *partitions.Service, runID string, timeout time.Duration) partitions.Run {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		run, err := svc.Get(context.Background(), runID)
		require.NoError(t, err)
		if run.Status != partitions.StatusRunning {
			return *run
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("run %s did not settle within %v", runID, timeout)
	return partitions.Run{}
}

// indexExistsAndValid reports whether Postgres considers the index usable. A
// rebuild that reports success while leaving indisvalid false is the failure
// this guards.
func indexExistsAndValid(t *testing.T, env *E2ETestEnv, name string) bool {
	t.Helper()

	var valid bool
	err := env.App.DB.GetDB().QueryRowContext(context.Background(), `
		SELECT i.indisvalid
		FROM pg_class c
		JOIN pg_index i ON i.indexrelid = c.oid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1 AND n.nspname = 'convoy'`, name).Scan(&valid)
	if err != nil {
		return false
	}
	return valid
}

// scrapeMetrics reads /metrics off the running server. The endpoint carries no
// auth, so this is a plain client scrape like Prometheus performs.
func scrapeMetrics(t *testing.T, env *E2ETestEnv) string {
	t.Helper()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(env.ServerURL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	return string(body)
}

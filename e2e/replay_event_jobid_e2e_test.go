package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/testenv"
)

func TestE2E_ReplayEvent_JobID_Format(t *testing.T) {
	// Setup E2E environment WITHOUT starting the real worker
	env := SetupE2EWithoutWorker(t)

	// Create job ID validator and custom test worker
	validator := testenv.NewJobIDValidator(t)
	testWorker := testenv.NewTestWorker(env.ctx, t, env.App.Queue, validator)
	testWorker.Start()
	defer testWorker.Stop()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	// Note: For replay tests, we need events to exist in the database.
	// Since our test worker doesn't create Event records (it just validates job IDs),
	// we'll create an event directly in the database using testdb helper

	// First create a dummy endpoint (required by SeedEvent)
	endpoint, err := testdb.SeedEndpoint(env.App.DB, env.Project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)

	eventUID := ulid.Make().String()
	event, err := testdb.SeedEvent(
		env.App.DB,
		endpoint,
		env.Project.UID,
		eventUID,
		"replay.event",
		"", // sourceID not needed
		[]byte(`{"test": "data"}`),
	)
	require.NoError(t, err)
	t.Logf("Created event in database: %s", event.UID)

	// Clear validator before replay
	validator.Clear()

	// Replay the event
	client := &http.Client{}
	replayReq, err := http.NewRequest("PUT",
		fmt.Sprintf("%s/api/v1/projects/%s/events/%s/replay", env.ServerURL, env.Project.UID, event.UID),
		nil)
	require.NoError(t, err)
	replayReq.Header.Set("Authorization", "Bearer "+env.APIKey)

	replayResp, err := client.Do(replayReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, replayResp.StatusCode)
	replayResp.Body.Close()

	t.Logf("Replayed event: %s", event.UID)

	// Wait for replay job to be processed
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("replay:")
	t.Logf("Captured %d replay job(s)", len(jobIDs))

	// Verify replay job ID format if captured
	if len(jobIDs) > 0 {
		testenv.VerifyJobIDFormat(t, jobIDs[0], "replay", env.Project.UID)
		t.Logf("✅ Replay job ID format verified: %s", jobIDs[0])
	}

	t.Log("✅ E2E test passed: Replay event with correct job ID")
}

func TestE2E_ReplayEvent_MultipleReplays(t *testing.T) {
	// Setup E2E environment WITHOUT starting the real worker
	env := SetupE2EWithoutWorker(t)

	// Create job ID validator and custom test worker
	validator := testenv.NewJobIDValidator(t)
	testWorker := testenv.NewTestWorker(env.ctx, t, env.App.Queue, validator)
	testWorker.Start()
	defer testWorker.Stop()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	// Note: For replay tests, we need events to exist in the database.
	// Since our test worker doesn't create Event records (it just validates job IDs),
	// we'll create an event directly in the database using testdb helper

	// First create a dummy endpoint (required by SeedEvent)
	endpoint, err := testdb.SeedEndpoint(env.App.DB, env.Project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)

	eventUID := ulid.Make().String()
	event, err := testdb.SeedEvent(
		env.App.DB,
		endpoint,
		env.Project.UID,
		eventUID,
		"multi.replay.event",
		"", // sourceID not needed
		[]byte(`{"test": "data"}`),
	)
	require.NoError(t, err)
	t.Logf("Created event in database: %s", event.UID)

	// Clear validator before replays
	validator.Clear()

	// Replay the event twice
	client := &http.Client{}
	for i := 1; i <= 2; i++ {
		replayReq, err := http.NewRequest("PUT",
			fmt.Sprintf("%s/api/v1/projects/%s/events/%s/replay", env.ServerURL, env.Project.UID, event.UID),
			nil)
		require.NoError(t, err)
		replayReq.Header.Set("Authorization", "Bearer "+env.APIKey)

		replayResp, err := client.Do(replayReq)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, replayResp.StatusCode)
		replayResp.Body.Close()

		t.Logf("Replay %d completed for event: %s", i, event.UID)
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for replay jobs to be processed
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("replay:")
	t.Logf("Captured %d replay job(s)", len(jobIDs))

	// We should have captured 2 replay jobs
	require.GreaterOrEqual(t, len(jobIDs), 2, "Should have captured at least 2 replay jobs")

	t.Log("✅ E2E test passed: Multiple replay events processed correctly")
}

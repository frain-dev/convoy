package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestE2E_DynamicEvent_JobID_Format(t *testing.T) {
	// Setup E2E environment WITHOUT starting the real worker
	env := SetupE2EWithoutWorker(t)

	// Create job ID validator and custom test worker
	validator := NewJobIDValidator(t)
	testWorker := NewTestWorker(env.ctx, t, env.App.Queue, validator)
	testWorker.Start()
	defer testWorker.Stop()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", 19980)

	// Clear validator before sending event
	validator.Clear()

	// Send dynamic event via API (creates endpoint on the fly)
	traceID := "dynamic-jobid-" + ulid.Make().String()
	dynamicPayload := map[string]interface{}{
		"url":        webhookURL,
		"secret":     "dynamic-secret",
		"event_type": "dynamic.event",
		"data": map[string]string{
			"traceId": traceID,
			"message": "Dynamic event creates endpoint inline",
		},
	}

	body, err := json.Marshal(dynamicPayload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/api/v1/projects/%s/events/dynamic", env.ServerURL, env.Project.UID),
		bytes.NewBuffer(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	t.Logf("Sent dynamic event with traceId: %s", traceID)

	// Wait for job to be processed
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("dynamic:")
	require.NotEmpty(t, jobIDs, "Should have captured at least one dynamic job ID")

	t.Logf("Captured dynamic job IDs: %v", jobIDs)

	// Verify job ID format: dynamic:{projectID}:{eventID}
	VerifyJobIDFormat(t, jobIDs[0], "dynamic", env.Project.UID)

	t.Logf("✅ Job ID format verified: %s", jobIDs[0])

	t.Log("✅ E2E test passed: Dynamic event with correct job ID")
}

func TestE2E_DynamicEvent_MultipleEventTypes(t *testing.T) {
	// Setup E2E environment WITHOUT starting the real worker
	env := SetupE2EWithoutWorker(t)

	// Create job ID validator and custom test worker
	validator := NewJobIDValidator(t)
	testWorker := NewTestWorker(env.ctx, t, env.App.Queue, validator)
	testWorker.Start()
	defer testWorker.Stop()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	webhookURL1 := fmt.Sprintf("http://localhost:%d/webhook", 19981)
	webhookURL2 := fmt.Sprintf("http://localhost:%d/webhook", 19982)
	webhookURL3 := fmt.Sprintf("http://localhost:%d/webhook", 19983)

	// Clear validator before sending events
	validator.Clear()

	// Send multiple dynamic events with different event types
	traceID1 := "dynamic-multi-1-" + ulid.Make().String()
	traceID2 := "dynamic-multi-2-" + ulid.Make().String()
	traceID3 := "dynamic-multi-3-" + ulid.Make().String()

	// Dynamic event 1
	payload1 := map[string]interface{}{
		"url":        webhookURL1,
		"secret":     "dynamic-secret-1",
		"event_type": "dynamic.type.one",
		"data":       map[string]string{"traceId": traceID1},
	}

	// Dynamic event 2
	payload2 := map[string]interface{}{
		"url":        webhookURL2,
		"secret":     "dynamic-secret-2",
		"event_type": "dynamic.type.two",
		"data":       map[string]string{"traceId": traceID2},
	}

	// Dynamic event 3
	payload3 := map[string]interface{}{
		"url":        webhookURL3,
		"secret":     "dynamic-secret-3",
		"event_type": "dynamic.type.four",
		"data":       map[string]string{"traceId": traceID3},
	}

	// Send all events
	for i, payload := range []map[string]interface{}{payload1, payload2, payload3} {
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req, err := http.NewRequest("POST",
			fmt.Sprintf("%s/api/v1/projects/%s/events/dynamic", env.ServerURL, env.Project.UID),
			bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.APIKey)

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()

		t.Logf("Sent dynamic event %d", i+1)
	}

	// Wait for jobs to be processed
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("dynamic:")
	t.Logf("Captured %d dynamic job(s)", len(jobIDs))
	require.GreaterOrEqual(t, len(jobIDs), 3, "Should have captured at least 3 dynamic jobs")

	t.Log("✅ E2E test passed: Multiple dynamic events processed correctly")
}

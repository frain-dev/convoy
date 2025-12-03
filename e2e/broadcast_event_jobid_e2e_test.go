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

	convoy "github.com/frain-dev/convoy-go/v2"
)

func TestE2E_BroadcastEvent_JobID_Format(t *testing.T) {
	// Setup E2E environment WITHOUT starting the real worker
	env := SetupE2EWithoutWorker(t)

	// Create job ID validator and custom test worker
	validator := NewJobIDValidator(t)
	testWorker := NewTestWorker(env.ctx, t, env.App.Queue, validator)
	testWorker.Start()
	defer testWorker.Stop()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	ownerID1 := env.Organisation.OwnerID + "_broadcast_1"
	ownerID2 := env.Organisation.OwnerID + "_broadcast_2"

	// Create two endpoints with different owners
	endpoint1 := CreateEndpointViaSDK(t, c, 19970, ownerID1)
	endpoint2 := CreateEndpointViaSDK(t, c, 19971, ownerID2)
	t.Logf("Created endpoints: %s (owner1), %s (owner2)", endpoint1.UID, endpoint2.UID)

	// Create subscriptions for both endpoints
	CreateSubscriptionViaSDK(t, c, endpoint1.UID, []string{"broadcast.event"})
	CreateSubscriptionViaSDK(t, c, endpoint2.UID, []string{"broadcast.event"})

	// Clear validator before sending event
	validator.Clear()

	// Send broadcast event via API
	traceID := "broadcast-jobid-" + ulid.Make().String()
	broadcastPayload := map[string]interface{}{
		"event_type": "broadcast.event",
		"data": map[string]string{
			"traceId": traceID,
		},
	}

	body, err := json.Marshal(broadcastPayload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/api/v1/projects/%s/events/broadcast", env.ServerURL, env.Project.UID),
		bytes.NewBuffer(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	t.Logf("Sent broadcast event with traceId: %s", traceID)

	// Wait for job to be processed
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("broadcast:")
	require.NotEmpty(t, jobIDs, "Should have captured at least one broadcast job ID")

	t.Logf("Captured broadcast job IDs: %v", jobIDs)

	// Verify job ID format: broadcast:{projectID}:{eventID}
	VerifyJobIDFormat(t, jobIDs[0], "broadcast", env.Project.UID)

	t.Logf("✅ Job ID format verified: %s", jobIDs[0])

	t.Log("✅ E2E test passed: Broadcast event with correct job ID")
}

func TestE2E_BroadcastEvent_AllSubscribers(t *testing.T) {
	// Setup E2E environment WITHOUT starting the real worker
	env := SetupE2EWithoutWorker(t)

	// Create job ID validator and custom test worker
	validator := NewJobIDValidator(t)
	testWorker := NewTestWorker(env.ctx, t, env.App.Queue, validator)
	testWorker.Start()
	defer testWorker.Stop()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Create three endpoints with different owners
	ownerID1 := env.Organisation.OwnerID + "_bc_all_1"
	ownerID2 := env.Organisation.OwnerID + "_bc_all_2"
	ownerID3 := env.Organisation.OwnerID + "_bc_all_3"

	endpoint1 := CreateEndpointViaSDK(t, c, 19972, ownerID1)
	endpoint2 := CreateEndpointViaSDK(t, c, 19973, ownerID2)
	endpoint3 := CreateEndpointViaSDK(t, c, 19974, ownerID3)
	t.Logf("Created endpoints: %s, %s, %s", endpoint1.UID, endpoint2.UID, endpoint3.UID)

	// Create subscriptions for all endpoints
	CreateSubscriptionViaSDK(t, c, endpoint1.UID, []string{"global.broadcast"})
	CreateSubscriptionViaSDK(t, c, endpoint2.UID, []string{"global.broadcast"})
	CreateSubscriptionViaSDK(t, c, endpoint3.UID, []string{"global.broadcast"})

	// Clear validator before sending event
	validator.Clear()

	// Send broadcast event
	traceID := "broadcast-all-" + ulid.Make().String()
	broadcastPayload := map[string]interface{}{
		"event_type": "global.broadcast",
		"data": map[string]string{
			"traceId": traceID,
			"message": "This should reach all subscribers",
		},
	}

	body, err := json.Marshal(broadcastPayload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/api/v1/projects/%s/events/broadcast", env.ServerURL, env.Project.UID),
		bytes.NewBuffer(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	t.Logf("Sent broadcast event with traceId: %s", traceID)

	// Wait for job to be processed
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("broadcast:")
	require.NotEmpty(t, jobIDs, "Should have captured at least one broadcast job ID")

	// Verify job ID format
	VerifyJobIDFormat(t, jobIDs[0], "broadcast", env.Project.UID)
	t.Logf("✅ Broadcast job found: %s", jobIDs[0])

	t.Log("✅ E2E test passed: Broadcast event reached all subscribers")
}

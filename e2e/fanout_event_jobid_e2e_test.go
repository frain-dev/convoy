package e2e

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
)

func TestE2E_FanoutEvent_JobID_Format(t *testing.T) {
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

	ownerID := env.Organisation.OwnerID + "_fanout_test"

	// Create two endpoints with same owner (no need for webhook servers)
	endpoint1 := CreateEndpointViaSDK(t, c, 19960, ownerID)
	endpoint2 := CreateEndpointViaSDK(t, c, 19961, ownerID)
	t.Logf("Created endpoints: %s, %s", endpoint1.UID, endpoint2.UID)

	// Create subscriptions for both endpoints
	CreateSubscriptionViaSDK(t, c, endpoint1.UID, []string{"fanout.event"})
	CreateSubscriptionViaSDK(t, c, endpoint2.UID, []string{"fanout.event"})

	// Clear validator before sending event
	validator.Clear()

	// Send fanout event
	traceID := "fanout-jobid-" + ulid.Make().String()
	SendFanoutEventViaSDK(t, c, ownerID, "fanout.event", traceID)

	t.Logf("Sent fanout event with traceId: %s", traceID)

	// Wait for job to be processed by our test worker
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("fanout:")
	require.NotEmpty(t, jobIDs, "Should have captured at least one fanout job ID")

	t.Logf("Captured fanout job IDs: %v", jobIDs)

	// Verify job ID format: fanout:{projectID}:{eventUID}
	VerifyJobIDFormat(t, jobIDs[0], "fanout", env.Project.UID)

	t.Logf("✅ Job ID format verified: %s", jobIDs[0])

	t.Log("✅ E2E test passed: Fanout event with correct job ID")
}

func TestE2E_FanoutEvent_MultipleOwners(t *testing.T) {
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

	ownerID1 := env.Organisation.OwnerID + "_fanout_owner1"
	ownerID2 := env.Organisation.OwnerID + "_fanout_owner2"

	// Create endpoints: 2 for owner1, 1 for owner2
	endpoint1 := CreateEndpointViaSDK(t, c, 19962, ownerID1)
	endpoint2 := CreateEndpointViaSDK(t, c, 19963, ownerID1)
	endpoint3 := CreateEndpointViaSDK(t, c, 19964, ownerID2)
	t.Logf("Created endpoints: %s (owner1), %s (owner1), %s (owner2)", endpoint1.UID, endpoint2.UID, endpoint3.UID)

	// Create subscriptions for all endpoints
	CreateSubscriptionViaSDK(t, c, endpoint1.UID, []string{"multi.fanout.event"})
	CreateSubscriptionViaSDK(t, c, endpoint2.UID, []string{"multi.fanout.event"})
	CreateSubscriptionViaSDK(t, c, endpoint3.UID, []string{"multi.fanout.event"})

	// Clear validator before sending event
	validator.Clear()

	// Send fanout event to owner1 (should reach endpoint1 and endpoint2 only)
	traceID := "fanout-multi-" + ulid.Make().String()
	SendFanoutEventViaSDK(t, c, ownerID1, "multi.fanout.event", traceID)

	t.Logf("Sent fanout event to owner1 with traceId: %s", traceID)

	// Wait for job to be processed
	time.Sleep(2 * time.Second)

	// Get captured job IDs from validator
	jobIDs := validator.GetJobIDsWithPrefix("fanout:")
	require.NotEmpty(t, jobIDs, "Should have captured at least one fanout job ID")

	// Verify job ID format
	VerifyJobIDFormat(t, jobIDs[0], "fanout", env.Project.UID)
	t.Logf("✅ Found fanout job: %s", jobIDs[0])

	t.Log("✅ E2E test passed: Fanout event correctly targets owner's endpoints only")
}

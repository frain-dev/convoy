package e2e

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
)

func TestE2E_FanOutEvent_AllSubscriptions(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(6) // Expecting 6 webhooks (2 events × 3 endpoints)

	// Start mock webhook servers on different ports
	port1 := 19911
	port2 := 19912
	port3 := 19913
	StartMockWebhookServer(t, manifest, done, &counter, port1)
	StartMockWebhookServer(t, manifest, done, &counter, port2)
	StartMockWebhookServer(t, manifest, done, &counter, port3)

	ownerID := env.Organisation.OwnerID + "_e2e_fanout_0"

	// Create 3 endpoints pointing to different mock servers
	endpoint1 := CreateEndpointViaSDK(t, c, port1, ownerID)
	t.Logf("Created endpoint1: %s at http://%s:%d/webhook", endpoint1.UID, "localhost", port1)

	endpoint2 := CreateEndpointViaSDK(t, c, port2, ownerID)
	t.Logf("Created endpoint2: %s at http://%s:%d/webhook", endpoint2.UID, "localhost", port2)

	endpoint3 := CreateEndpointViaSDK(t, c, port3, ownerID)
	t.Logf("Created endpoint3: %s at http://%s:%d/webhook", endpoint3.UID, "localhost", port3)

	// Create subscriptions with wildcard filter for all endpoints
	subscription1 := CreateSubscriptionViaSDK(t, c, endpoint1.UID, []string{"*"})
	t.Logf("Created subscription1: %s with wildcard filter", subscription1.UID)

	subscription2 := CreateSubscriptionViaSDK(t, c, endpoint2.UID, []string{"*"})
	t.Logf("Created subscription2: %s with wildcard filter", subscription2.UID)

	subscription3 := CreateSubscriptionViaSDK(t, c, endpoint3.UID, []string{"*"})
	t.Logf("Created subscription3: %s with wildcard filter", subscription3.UID)

	// Send two fanout events
	traceId1 := "e2e-fanout-all-0-" + ulid.Make().String()
	traceId2 := "e2e-fanout-all-1-" + ulid.Make().String()

	SendFanoutEventViaSDK(t, c, ownerID, "test.fanout.event", traceId1)
	t.Logf("Sent fanout event with traceId: %s", traceId1)

	SendFanoutEventViaSDK(t, c, ownerID, "test.another.fanout.event", traceId2)
	t.Logf("Sent fanout event with traceId: %s", traceId2)

	// Wait for webhooks to be delivered
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify webhooks were received on all 3 endpoints
	webhookURL1 := fmt.Sprintf("http://%s:%d/webhook", "localhost", port1)
	webhookURL2 := fmt.Sprintf("http://%s:%d/webhook", "localhost", port2)
	webhookURL3 := fmt.Sprintf("http://%s:%d/webhook", "localhost", port3)

	hits1 := manifest.ReadEndpoint(webhookURL1)
	hits2 := manifest.ReadEndpoint(webhookURL2)
	hits3 := manifest.ReadEndpoint(webhookURL3)

	require.Equal(t, 2, hits1, "Endpoint 1 should have received 2 webhooks")
	require.Equal(t, 2, hits2, "Endpoint 2 should have received 2 webhooks")
	require.Equal(t, 2, hits3, "Endpoint 3 should have received 2 webhooks")

	// Verify both events were delivered
	event1Count := 0
	event2Count := 0

	for key := range manifest.events {
		if contains(key, traceId1) {
			event1Count = manifest.events[key]
		}

		if contains(key, traceId2) {
			event2Count = manifest.events[key]
		}
	}

	require.Equal(t, 3, event1Count, "Event 1 should be delivered to 3 endpoints")
	require.Equal(t, 3, event2Count, "Event 2 should be delivered to 3 endpoints")

	t.Log("✅ E2E test passed: All fanout webhooks delivered successfully")
}

func TestE2E_FanOutEvent_MustMatchSubscription(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(3) // Expecting only 3 webhooks (1 matching event × 3 endpoints)

	// Start mock webhook servers on different ports
	port1 := 19914
	port2 := 19915
	port3 := 19916
	StartMockWebhookServer(t, manifest, done, &counter, port1)
	StartMockWebhookServer(t, manifest, done, &counter, port2)
	StartMockWebhookServer(t, manifest, done, &counter, port3)

	ownerID := env.Organisation.OwnerID + "_e2e_fanout_1"

	// Create 3 endpoints pointing to different mock servers
	endpoint1 := CreateEndpointViaSDK(t, c, port1, ownerID)
	t.Logf("Created endpoint1: %s at http://%s:%d/webhook", endpoint1.UID, "localhost", port1)

	endpoint2 := CreateEndpointViaSDK(t, c, port2, ownerID)
	t.Logf("Created endpoint2: %s at http://%s:%d/webhook", endpoint2.UID, "localhost", port2)

	endpoint3 := CreateEndpointViaSDK(t, c, port3, ownerID)
	t.Logf("Created endpoint3: %s at http://%s:%d/webhook", endpoint3.UID, "localhost", port3)

	// Create subscriptions with specific filter for all endpoints
	subscription1 := CreateSubscriptionViaSDK(t, c, endpoint1.UID, []string{"invoice.created"})
	t.Logf("Created subscription1: %s with filter: invoice.created", subscription1.UID)

	subscription2 := CreateSubscriptionViaSDK(t, c, endpoint2.UID, []string{"invoice.created"})
	t.Logf("Created subscription2: %s with filter: invoice.created", subscription2.UID)

	subscription3 := CreateSubscriptionViaSDK(t, c, endpoint3.UID, []string{"invoice.created"})
	t.Logf("Created subscription3: %s with filter: invoice.created", subscription3.UID)

	// Send two fanout events - one matching, one not
	traceIdMismatch := "e2e-fanout-mismatch-" + ulid.Make().String()
	traceIdMatch := "e2e-fanout-match-" + ulid.Make().String()

	SendFanoutEventViaSDK(t, c, ownerID, "mismatched.event", traceIdMismatch)
	t.Logf("Sent mismatched fanout event with traceId: %s", traceIdMismatch)

	SendFanoutEventViaSDK(t, c, ownerID, "invoice.created", traceIdMatch)
	t.Logf("Sent matching fanout event with traceId: %s", traceIdMatch)

	// Wait for webhooks to be delivered (only the matching ones)
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify webhooks were received on all 3 endpoints (only 1 webhook each)
	webhookURL1 := fmt.Sprintf("http://%s:%d/webhook", "localhost", port1)
	webhookURL2 := fmt.Sprintf("http://%s:%d/webhook", "localhost", port2)
	webhookURL3 := fmt.Sprintf("http://%s:%d/webhook", "localhost", port3)

	hits1 := manifest.ReadEndpoint(webhookURL1)
	hits2 := manifest.ReadEndpoint(webhookURL2)
	hits3 := manifest.ReadEndpoint(webhookURL3)

	require.Equal(t, 1, hits1, "Endpoint 1 should have received 1 webhook (matching event)")
	require.Equal(t, 1, hits2, "Endpoint 2 should have received 1 webhook (matching event)")
	require.Equal(t, 1, hits3, "Endpoint 3 should have received 1 webhook (matching event)")

	// Verify only the matching event was delivered
	matchedCount := 0
	mismatchedCount := 0
	for key := range manifest.events {
		if contains(key, traceIdMatch) {
			matchedCount = manifest.events[key]
		}
		if contains(key, traceIdMismatch) {
			mismatchedCount = manifest.events[key]
		}
	}

	require.Equal(t, 3, matchedCount, "Matching event should be delivered to 3 endpoints")
	require.Equal(t, 0, mismatchedCount, "Mismatched event should NOT be delivered")

	t.Log("✅ E2E test passed: Fanout subscription filtering works correctly")
}

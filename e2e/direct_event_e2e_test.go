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

func TestE2E_DirectEvent_AllSubscriptions(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(2) // Expecting 2 webhooks

	// Start mock webhook server
	port := 19909
	StartMockWebhookServer(t, manifest, done, &counter, port)

	ownerID := env.Organisation.OwnerID + "_e2e_0"

	// Create endpoint pointing to mock server
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)
	t.Logf("Created endpoint: %s at http://localhost:%d/webhook", endpoint.UID, port)

	// Create subscription with wildcard filter
	subscription := CreateSubscriptionViaSDK(t, c, endpoint.UID, []string{"*"})
	t.Logf("Created subscription: %s with wildcard filter", subscription.UID)

	// Send two events
	traceId1 := "e2e-direct-all-0-" + ulid.Make().String()
	traceId2 := "e2e-direct-all-1-" + ulid.Make().String()

	SendEventViaSDK(t, c, endpoint.UID, "test.event", traceId1)
	t.Logf("Sent event with traceId: %s", traceId1)

	SendEventViaSDK(t, c, endpoint.UID, "test.another.event", traceId2)
	t.Logf("Sent event with traceId: %s", traceId2)

	// Wait for webhooks to be delivered
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify webhooks were received
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 2, hits, "Should have received 2 webhooks")

	// Verify both events were delivered
	event1Count := 0
	event2Count := 0
	for key := range manifest.events {
		if contains(key, traceId1) {
			event1Count++
		}
		if contains(key, traceId2) {
			event2Count++
		}
	}

	require.Equal(t, 1, event1Count, "Event 1 should be delivered once")
	require.Equal(t, 1, event2Count, "Event 2 should be delivered once")

	t.Log("✅ E2E test passed: All webhooks delivered successfully")
}

func TestE2E_DirectEvent_MustMatchSubscription(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expecting only 1 webhook (matching event)

	// Start mock webhook server
	port := 19910
	StartMockWebhookServer(t, manifest, done, &counter, port)

	ownerID := env.Organisation.OwnerID + "_e2e_1"

	// Create endpoint pointing to mock server
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)
	t.Logf("Created endpoint: %s at http://localhost:%d/webhook", endpoint.UID, port)

	// Create subscription with specific filter
	subscription := CreateSubscriptionViaSDK(t, c, endpoint.UID, []string{"invoice.created"})
	t.Logf("Created subscription: %s with filter: invoice.created", subscription.UID)

	// Send two events - one matching, one not
	traceIdMismatch := "e2e-direct-mismatch-" + ulid.Make().String()
	traceIdMatch := "e2e-direct-match-" + ulid.Make().String()

	SendEventViaSDK(t, c, endpoint.UID, "mismatched.event", traceIdMismatch)
	t.Logf("Sent mismatched event with traceId: %s", traceIdMismatch)

	SendEventViaSDK(t, c, endpoint.UID, "invoice.created", traceIdMatch)
	t.Logf("Sent matching event with traceId: %s", traceIdMatch)

	// Wait for the webhooks to be delivered (only the matching one)
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify only 1 webhook was received
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 1, hits, "Should have received only 1 webhook (matching event)")

	// Verify only the matching event was delivered
	matchedCount := 0
	mismatchedCount := 0
	for key := range manifest.events {
		if contains(key, traceIdMatch) {
			matchedCount++
		}
		if contains(key, traceIdMismatch) {
			mismatchedCount++
		}
	}

	require.Equal(t, 1, matchedCount, "Matching event should be delivered")
	require.Equal(t, 0, mismatchedCount, "Mismatched event should NOT be delivered")

	t.Log("✅ E2E test passed: Subscription filtering works correctly")
}

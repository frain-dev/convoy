package e2e

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"

	"github.com/frain-dev/convoy/config"
)

func TestE2E_RedisSentinel(t *testing.T) {
	scheme := os.Getenv("CONVOY_REDIS_SCHEME")
	if scheme != config.RedisSentinelScheme && scheme != "redis-sentinel" {
		t.Skipf("Skipping Redis Sentinel E2E test because CONVOY_REDIS_SCHEME is not set to '%s' (current: '%s'). "+
			"To run this test, ensure the Sentinel stack is running and necessary environment variables are set.",
			config.RedisSentinelScheme, scheme)
	}

	t.Log("Starting Redis Sentinel E2E Integration Test")

	env := SetupE2E(t)
	require.NotNil(t, env, "E2E environment should be successfully initialized")

	client := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	manifest := NewEventManifest()
	webhookCh := make(chan bool, 1)

	var expectedDeliveries atomic.Int64
	expectedDeliveries.Store(1)

	mockWebhookPort := 19915
	StartMockWebhookServer(t, manifest, webhookCh, &expectedDeliveries, mockWebhookPort)

	ownerID := fmt.Sprintf("%s_e2e_sentinel", env.Organisation.OwnerID)

	endpoint := CreateEndpointViaSDK(t, client, mockWebhookPort, ownerID)
	require.NotEmpty(t, endpoint.UID, "Endpoint creation should return a valid UID")
	t.Logf("Created test endpoint %s listening at http://localhost:%d/webhook", endpoint.UID, mockWebhookPort)

	subscription := CreateSubscriptionViaSDK(t, client, endpoint.UID, []string{"*"})
	require.NotEmpty(t, subscription.UID, "Subscription creation should return a valid UID")
	t.Logf("Created subscription %s with wildcard event filter", subscription.UID)

	traceID := fmt.Sprintf("e2e-sentinel-%s", ulid.Make().String())
	SendEventViaSDK(t, client, endpoint.UID, "test.sentinel.event", traceID)
	t.Logf("Dispatched event with traceId %s. Waiting for worker to process through Sentinel-managed queue...", traceID)

	WaitForWebhooks(t, webhookCh, 30*time.Second)

	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", mockWebhookPort)
	deliveryCountForUrl := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 1, deliveryCountForUrl, "Mock server should have recorded exactly 1 webhook delivery for the target URL")

	matchedEventCount := 0
	for key := range manifest.events {
		if contains(key, traceID) {
			matchedEventCount++
		}
	}
	require.Equal(t, 1, matchedEventCount, "The specific event payload (identified by traceID) should be recorded exactly once")

	t.Log("Redis Sentinel E2E test passed: Messages successfully routed through Sentinel cluster.")

	t.Cleanup(func() {
		t.Log("Cleaning up Convoy test resources...")
		err := client.Subscriptions.Delete(context.Background(), subscription.UID)
		if err != nil {
			t.Logf("Failed to delete subscription %s: %v", subscription.UID, err)
		}

		err = client.Endpoints.Delete(context.Background(), endpoint.UID, nil)
		if err != nil {
			t.Logf("Failed to delete endpoint %s: %v", endpoint.UID, err)
		}
	})
}

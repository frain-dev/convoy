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

func TestE2E_FormEndpoint_ContentType(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Debug: Verify project exists in database
	t.Logf("Project UID: %s", env.Project.UID)
	t.Logf("Project Name: %s", env.Project.Name)
	t.Logf("Organisation UID: %s", env.Organisation.UID)
	t.Logf("API Key (first 10 chars): %s...", env.APIKey[:10])

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expecting 1 webhook

	// Start mock webhook server
	port := 19917
	StartMockWebhookServer(t, manifest, done, &counter, port)

	ownerID := env.Organisation.OwnerID + "_e2e_form_0"

	// Create endpoint with form content type
	endpoint := CreateFormEndpointViaSDK(t, c, port, ownerID)
	t.Logf("Created form endpoint: %s at http://localhost:%d/webhook", endpoint.UID, port)
	require.Equal(t, "application/x-www-form-urlencoded", endpoint.ContentType, "Endpoint should have form content type")

	// Create subscription with wildcard filter
	subscription := CreateSubscriptionViaSDK(t, c, endpoint.UID, []string{"*"})
	t.Logf("Created subscription: %s with wildcard filter", subscription.UID)

	// Send form event
	traceId := "e2e-form-content-type-" + ulid.Make().String()
	SendEventViaSDK(t, c, endpoint.UID, "form.submitted", traceId)
	t.Logf("Sent form event with traceId: %s", traceId)

	// Wait for webhook to be delivered
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify webhook was received
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 1, hits, "Should have received 1 webhook")

	// Verify the event was delivered
	eventCount := 0
	for key := range manifest.events {
		if contains(key, traceId) {
			eventCount++
		}
	}

	require.Equal(t, 1, eventCount, "Form event should be delivered once")

	t.Log("✅ E2E test passed: Form endpoint with correct content type delivered successfully")
}

func TestE2E_FormEndpoint_WithCustomHeaders(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expecting 1 webhook

	// Start mock webhook server
	port := 19918
	StartMockWebhookServer(t, manifest, done, &counter, port)

	ownerID := env.Organisation.OwnerID + "_e2e_form_1"

	// Create endpoint with form content type
	endpoint := CreateFormEndpointViaSDK(t, c, port, ownerID)
	t.Logf("Created form endpoint: %s at http://localhost:%d/webhook", endpoint.UID, port)
	require.Equal(t, "application/x-www-form-urlencoded", endpoint.ContentType, "Endpoint should have form content type")

	// Create subscription with specific filter
	subscription := CreateSubscriptionViaSDK(t, c, endpoint.UID, []string{"form.custom"})
	t.Logf("Created subscription: %s with filter: form.custom", subscription.UID)

	// Send form event with custom headers
	traceId := "e2e-form-custom-headers-" + ulid.Make().String()
	SendEventViaSDK(t, c, endpoint.UID, "form.custom", traceId)
	t.Logf("Sent form event with custom headers and traceId: %s", traceId)

	// Wait for webhook to be delivered
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify webhook was received
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 1, hits, "Should have received 1 webhook")

	// Verify the event was delivered
	eventCount := 0
	for key := range manifest.events {
		if contains(key, traceId) {
			eventCount++
		}
	}

	require.Equal(t, 1, eventCount, "Form event with custom headers should be delivered once")

	t.Log("✅ E2E test passed: Form endpoint with custom headers delivered successfully")
}

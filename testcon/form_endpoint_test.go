//go:build docker_testcon
// +build docker_testcon

package testcon

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
)

func (d *DockerE2EIntegrationTestSuite) Test_FormEndpoint_Success_ContentType() {
	ctx := context.Background()
	t := d.T()
	ownerID := d.DefaultOrg.OwnerID + "_form_0"

	var ports = []int{9920}

	c, done := d.initAndStartServersWithType(t, ports, 1, "form")

	// Create endpoint with ContentType
	endpoint := createFormEndpoints(t, ctx, c, ports, ownerID)[0]

	// Verify the endpoint was created with the correct ContentType
	t.Logf("Created endpoint UID: %s", endpoint.UID)
	t.Logf("Created endpoint ContentType: %s", endpoint.ContentType)
	require.Equal(t, "application/x-www-form-urlencoded", endpoint.ContentType, "Endpoint should have form content type")

	// Verify the endpoint was saved correctly in the database
	dbEndpoint, err := c.Endpoints.Find(ctx, endpoint.UID, &convoy.EndpointParams{})
	require.NoError(t, err, "Failed to fetch endpoint from database")
	t.Logf("Database endpoint ContentType: %s", dbEndpoint.ContentType)
	require.Equal(t, "application/x-www-form-urlencoded", dbEndpoint.ContentType, "Database endpoint should have form content type")

	createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"*"})

	traceId := "event-form-content-type-" + ulid.Make().String()

	// Send form data event
	require.NoError(t, sendFormEvent(ctx, c, endpoint.UID, "form.submitted", traceId))

	assertEventCameThrough(t, done, []*convoy.EndpointResponse{endpoint}, []string{traceId}, []string{})

	// Assert form data was received correctly
	assertFormDataReceivedByEndpoint(t, traceId, "John Doe", "john@example.com")
}

func (d *DockerE2EIntegrationTestSuite) Test_FormEndpoint_Success_WithCustomHeaders() {
	ctx := context.Background()
	t := d.T()
	ownerID := d.DefaultOrg.OwnerID + "_form_2"

	var ports = []int{9923}

	c, done := d.initAndStartServersWithType(t, ports, 1, "form")

	endpoint := createFormEndpoints(t, ctx, c, ports, ownerID)[0]

	createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"form.custom"})

	traceId := "event-form-custom-headers-" + ulid.Make().String()

	// Send form event with custom headers
	require.NoError(t, sendFormEventWithHeaders(ctx, c, endpoint.UID, "form.custom", traceId))

	assertEventCameThrough(t, done, []*convoy.EndpointResponse{endpoint}, []string{traceId}, []string{})

	// Assert form data with custom headers was received correctly
	assertFormDataReceivedByEndpoint(t, traceId, "Jane Doe", "jane@example.com")
}

func createFormEndpoints(t *testing.T, ctx context.Context, c *convoy.Client, ports []int, ownerId string) []*convoy.EndpointResponse {
	endpoints := make([]*convoy.EndpointResponse, len(ports))
	for i, port := range ports {
		baseURL := fmt.Sprintf("http://%s:%d/api/convoy", GetOutboundIP().String(), port)

		// Create endpoint with ContentType
		body := &convoy.CreateEndpointRequest{
			Name:         "form-endpoint-" + ulid.Make().String(),
			URL:          baseURL,
			Secret:       "endpoint-secret",
			SupportEmail: "notifications@getconvoy.io",
			OwnerID:      ownerId,
			ContentType:  "application/x-www-form-urlencoded",
		}

		// Retry endpoint creation with exponential backoff
		var endpoint *convoy.EndpointResponse
		var err error
		maxRetries := 5
		retryDelay := 100 * time.Millisecond

		for attempt := 0; attempt < maxRetries; attempt++ {
			endpoint, err = c.Endpoints.Create(ctx, body, &convoy.EndpointParams{})
			if err == nil {
				break
			}

			if attempt < maxRetries-1 {
				t.Logf("Form endpoint creation attempt %d failed: %v, retrying in %v", attempt+1, err, retryDelay)
				time.Sleep(retryDelay)
				retryDelay *= 2 // Exponential backoff
			}
		}

		require.NoError(t, err, "Failed to create form endpoint after %d attempts", maxRetries)
		require.NotEmpty(t, endpoint.UID)

		endpoint.TargetUrl = baseURL
		endpoints[i] = endpoint
	}
	return endpoints
}

func sendFormEvent(ctx context.Context, c *convoy.Client, eUID string, eventType string, traceId string) error {
	// Send JSON data to Convoy API - Convoy will convert to form data when forwarding
	formData := map[string]interface{}{
		"traceId": traceId,
		"formData": map[string]interface{}{
			"user": map[string]string{
				"name":  "John Doe",
				"email": "john@example.com",
			},
		},
	}
	payload, err := json.Marshal(formData)
	if err != nil {
		return err
	}

	body := &convoy.CreateEventRequest{
		EventType:      eventType,
		EndpointID:     eUID,
		IdempotencyKey: eUID + ulid.Make().String(),
		Data:           payload,
	}
	return c.Events.Create(ctx, body)
}

func sendFormEventWithHeaders(ctx context.Context, c *convoy.Client, eUID string, eventType string, traceId string) error {
	// Create form data payload with custom headers
	formData := map[string]interface{}{
		"traceId": traceId,
		"formData": map[string]interface{}{
			"user": map[string]string{
				"name":  "Jane Doe",
				"email": "jane@example.com",
			},
		},
		"customHeaders": map[string]string{
			"X-Form-Source": "web",
			"X-User-Agent":  "test-client",
		},
	}
	payload, err := json.Marshal(formData)
	if err != nil {
		return err
	}

	body := &convoy.CreateEventRequest{
		EventType:      eventType,
		EndpointID:     eUID,
		IdempotencyKey: eUID + ulid.Make().String(),
		Data:           payload,
	}
	return c.Events.Create(ctx, body)
}

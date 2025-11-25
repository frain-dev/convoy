package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
)

// EventManifest tracks received webhooks for verification
type EventManifest struct {
	endpoints     map[string]int
	events        map[string]int
	endpointsLock sync.RWMutex
	eventsLock    sync.RWMutex
}

func NewEventManifest() *EventManifest {
	return &EventManifest{
		endpoints: make(map[string]int),
		events:    make(map[string]int),
	}
}

func (m *EventManifest) IncEndpoint(k string) {
	m.endpointsLock.Lock()
	defer m.endpointsLock.Unlock()
	m.endpoints[k]++
}

func (m *EventManifest) ReadEndpoint(k string) int {
	m.endpointsLock.RLock()
	defer m.endpointsLock.RUnlock()
	return m.endpoints[k]
}

func (m *EventManifest) IncEvent(k string) {
	m.eventsLock.Lock()
	defer m.eventsLock.Unlock()
	m.events[k]++
}

func (m *EventManifest) ReadEvent(k string) int {
	m.eventsLock.RLock()
	defer m.eventsLock.RUnlock()
	return m.events[k]
}

func (m *EventManifest) Reset() {
	m.endpointsLock.Lock()
	m.eventsLock.Lock()
	defer m.endpointsLock.Unlock()
	defer m.eventsLock.Unlock()
	m.endpoints = make(map[string]int)
	m.events = make(map[string]int)
}

// StartMockWebhookServer starts a mock HTTP server that receives and tracks webhooks
func StartMockWebhookServer(t *testing.T, manifest *EventManifest, done chan bool, counter *atomic.Int64, port int) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		endpoint := fmt.Sprintf("http://localhost:%d/webhook", port)
		manifest.IncEndpoint(endpoint)

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Logf("Error reading webhook body: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Parse the webhook payload based on content type
		contentType := r.Header.Get("Content-Type")
		var payload map[string]interface{}

		if contentType == "application/x-www-form-urlencoded" {
			// For form-encoded payloads, just track the raw body
			// Convoy sends form data, not JSON for form endpoints
			manifest.IncEvent(string(reqBody))
			t.Logf("Received form-encoded webhook on %s: %s", endpoint, string(reqBody))
		} else {
			// For JSON payloads, parse and track
			if err := json.Unmarshal(reqBody, &payload); err != nil {
				t.Logf("Error parsing webhook JSON: %v", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			eventJSON, _ := json.Marshal(payload)
			manifest.IncEvent(string(eventJSON))
			t.Logf("Received JSON webhook on %s: %s", endpoint, string(reqBody))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))

		// Decrement counter
		current := counter.Add(-1)
		if current <= 0 {
			select {
			case done <- true:
			default:
			}
		}
	})

	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Mock webhook server error on port %d: %v", port, err)
		}
	}()

	// Cleanup
	t.Cleanup(func() {
		server.Close()
	})

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
}

// GetOutboundIP returns the preferred outbound IP of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}

// CreateEndpointViaSDK creates an endpoint using the Convoy Go SDK
func CreateEndpointViaSDK(t *testing.T, c *convoy.Client, port int, ownerID string) *convoy.EndpointResponse {
	t.Helper()

	baseURL := fmt.Sprintf("http://localhost:%d/webhook", port)

	body := &convoy.CreateEndpointRequest{
		Name:         "endpoint-" + ulid.Make().String(),
		URL:          baseURL,
		Secret:       "endpoint-secret",
		SupportEmail: "test@example.com",
		OwnerID:      ownerID,
	}

	endpoint, err := c.Endpoints.Create(t.Context(), body, &convoy.EndpointParams{})
	require.NoError(t, err)
	require.NotEmpty(t, endpoint.UID)

	return endpoint
}

// CreateFormEndpointViaSDK creates an endpoint with form content type using the Convoy Go SDK
func CreateFormEndpointViaSDK(t *testing.T, c *convoy.Client, port int, ownerID string) *convoy.EndpointResponse {
	t.Helper()

	baseURL := fmt.Sprintf("http://localhost:%d/webhook", port)

	body := &convoy.CreateEndpointRequest{
		Name:         "form-endpoint-" + ulid.Make().String(),
		URL:          baseURL,
		Secret:       "endpoint-secret",
		SupportEmail: "test@example.com",
		OwnerID:      ownerID,
		ContentType:  "application/x-www-form-urlencoded",
	}

	endpoint, err := c.Endpoints.Create(t.Context(), body, &convoy.EndpointParams{})
	require.NoError(t, err)
	require.NotEmpty(t, endpoint.UID)

	return endpoint
}

// CreateSubscriptionViaSDK creates a subscription using the Convoy Go SDK
func CreateSubscriptionViaSDK(t *testing.T, c *convoy.Client, endpointUID string, eventTypes []string) *convoy.SubscriptionResponse {
	t.Helper()

	body := &convoy.CreateSubscriptionRequest{
		Name:       "subscription-" + ulid.Make().String(),
		EndpointID: endpointUID,
		FilterConfig: &convoy.FilterConfiguration{
			EventTypes: eventTypes,
		},
	}

	subscription, err := c.Subscriptions.Create(t.Context(), body)
	require.NoError(t, err)
	require.NotEmpty(t, subscription.UID)

	return subscription
}

// SendEventViaSDK sends an event using the Convoy Go SDK
func SendEventViaSDK(t *testing.T, c *convoy.Client, endpointUID, eventType, traceID string) {
	t.Helper()

	event := fmt.Sprintf(`{"traceId": %q}`, traceID)
	payload := []byte(event)

	body := &convoy.CreateEventRequest{
		EventType:      eventType,
		EndpointID:     endpointUID,
		IdempotencyKey: ulid.Make().String(),
		Data:           payload,
	}

	err := c.Events.Create(t.Context(), body)
	require.NoError(t, err)
}

// SendFanoutEventViaSDK sends a fanout event using the Convoy Go SDK
func SendFanoutEventViaSDK(t *testing.T, c *convoy.Client, ownerID, eventType, traceID string) {
	t.Helper()

	event := fmt.Sprintf(`{"traceId": %q}`, traceID)
	payload := []byte(event)

	body := &convoy.CreateFanoutEventRequest{
		EventType:      eventType,
		OwnerID:        ownerID,
		IdempotencyKey: ulid.Make().String(),
		Data:           payload,
	}

	err := c.Events.FanoutEvent(t.Context(), body)
	require.NoError(t, err)
}

// WaitForWebhooks waits for all expected webhooks to arrive
func WaitForWebhooks(t *testing.T, done chan bool, timeout time.Duration) {
	t.Helper()

	select {
	case <-done:
		t.Log("All webhooks received")
	case <-time.After(timeout):
		t.Fatal("Timeout waiting for webhooks")
	}
}

// VerifyWebhookDelivery verifies that webhooks were delivered correctly
func VerifyWebhookDelivery(t *testing.T, manifest *EventManifest, expectedEndpoints, expectedEvents []string) {
	t.Helper()

	// Verify endpoints received webhooks
	for _, endpoint := range expectedEndpoints {
		hits := manifest.ReadEndpoint(endpoint)
		require.Greater(t, hits, 0, "Endpoint %s should have received webhooks", endpoint)
	}

	// Verify events were delivered
	for _, eventData := range expectedEvents {
		// Find matching event in manifest
		found := false
		for key, count := range manifest.events {
			if contains(key, eventData) {
				require.Greater(t, count, 0, "Event %s should have been delivered", eventData)
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Event %s was not found in delivered events", eventData)
		}
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	//nolint:staticcheck // we don't want to use v2
	"cloud.google.com/go/pubsub"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/oklog/ulid/v2"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	convoy "github.com/frain-dev/convoy-go/v2"
	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/sources"
	"github.com/frain-dev/convoy/internal/subscriptions"
	"github.com/frain-dev/convoy/pkg/log"
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

// SendEventWithIdempotencyKey sends an event with a specific idempotency key using the Convoy Go SDK
func SendEventWithIdempotencyKey(t *testing.T, c *convoy.Client, endpointUID, eventType, traceID, idempotencyKey string) {
	t.Helper()

	event := fmt.Sprintf(`{"traceId": %q}`, traceID)
	payload := []byte(event)

	body := &convoy.CreateEventRequest{
		EventType:      eventType,
		EndpointID:     endpointUID,
		IdempotencyKey: idempotencyKey,
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

// AMQP Helper Functions

// CreateAMQPSource creates an AMQP source for testing
func CreateAMQPSource(t *testing.T, db *postgres.Postgres, ctx context.Context, project *datastore.Project,
	host string, port int, queue string, workers int, bodyFunction, headerFunction *string) *datastore.Source {
	t.Helper()

	vhost := "/"
	source := &datastore.Source{
		UID:          ulid.Make().String(),
		ProjectID:    project.UID,
		MaskID:       ulid.Make().String(),
		Name:         fmt.Sprintf("amqp-source-%s", ulid.Make().String()),
		Type:         datastore.PubSubSource,
		Provider:     datastore.GithubSourceProvider,
		IsDisabled:   false,
		BodyFunction: bodyFunction,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
		HeaderFunction: headerFunction,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.AmqpPubSub,
			Workers: workers,
			Amqp: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   host,
				Port:   fmt.Sprintf("%d", port),
				Queue:  queue,
				Auth: &datastore.AmqpCredentials{
					User:     "guest",
					Password: "guest",
				},
				Vhost: &vhost,
			},
		},
	}

	sourceRepo := sources.New(log.NewLogger(io.Discard), db)
	err := sourceRepo.CreateSource(ctx, source)
	require.NoError(t, err)

	return source
}

// UpdateAMQPSourcePort updates the port configuration for an AMQP source.
// This is useful when the RabbitMQ container is restarted and gets a new port.
func UpdateAMQPSourcePort(t *testing.T, db *postgres.Postgres, ctx context.Context, source *datastore.Source, newPort int) {
	t.Helper()

	source.PubSub.Amqp.Port = fmt.Sprintf("%d", newPort)
	sourceRepo := sources.New(log.NewLogger(io.Discard), db)
	err := sourceRepo.UpdateSource(ctx, source.ProjectID, source)
	require.NoError(t, err)
	t.Logf("Updated source %s port to %d", source.UID, newPort)
}

// PublishAMQPMessage publishes a message to RabbitMQ with headers
func PublishAMQPMessage(t *testing.T, host string, port int, queue string, body []byte, headers map[string]interface{}) {
	t.Helper()

	connStr := fmt.Sprintf("amqp://guest:guest@%s:%d/", host, port)
	conn, err := amqp091.Dial(connStr)
	require.NoError(t, err)
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Declare queue (idempotent) - must match AMQP consumer settings
	_, err = ch.QueueDeclare(
		queue, // name
		true,  // durable - must match consumer setting
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	require.NoError(t, err)

	// Convert headers to AMQP table
	amqpHeaders := make(amqp091.Table)
	for k, v := range headers {
		amqpHeaders[k] = v
	}

	err = ch.Publish(
		"",    // exchange
		queue, // routing key
		false, // mandatory
		false, // immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     amqpHeaders,
		},
	)
	require.NoError(t, err)
	t.Logf("Published AMQP message to queue %s", queue)
}

// PublishSingleAMQPMessage publishes a single-type AMQP message
func PublishSingleAMQPMessage(t *testing.T, host string, port int, queue, endpointID, eventType string, data map[string]interface{}, customHeaders map[string]string) {
	t.Helper()

	payload := map[string]interface{}{
		"endpoint_id":     endpointID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	if customHeaders != nil {
		payload["custom_headers"] = customHeaders
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	headers := map[string]interface{}{
		"x-convoy-message-type": "single",
	}

	PublishAMQPMessage(t, host, port, queue, body, headers)
}

// TryPublishAMQPMessage attempts to publish a single-type AMQP message and returns an error if it fails.
// This is useful for testing reconnection scenarios where we need to retry publishing.
func TryPublishAMQPMessage(t *testing.T, host string, port int, queue, endpointID, eventType string, data map[string]interface{}, customHeaders map[string]string) error {
	t.Helper()

	payload := map[string]interface{}{
		"endpoint_id":     endpointID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	if customHeaders != nil {
		payload["custom_headers"] = customHeaders
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	headers := map[string]interface{}{
		"x-convoy-message-type": "single",
	}

	return TryPublishAMQPMessageRaw(host, port, queue, body, headers)
}

// TryPublishAMQPMessageRaw attempts to publish a raw AMQP message and returns an error if it fails.
func TryPublishAMQPMessageRaw(host string, port int, queue string, body []byte, headers map[string]interface{}) error {
	connStr := fmt.Sprintf("amqp://guest:guest@%s:%d/", host, port)
	conn, err := amqp091.Dial(connStr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()

	// Declare queue (idempotent)
	_, err = ch.QueueDeclare(
		queue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Convert headers to AMQP table
	amqpHeaders := make(amqp091.Table)
	for k, v := range headers {
		amqpHeaders[k] = v
	}

	err = ch.Publish(
		"",    // exchange
		queue, // routing key
		false, // mandatory
		false, // immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     amqpHeaders,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

// PublishFanoutAMQPMessage publishes a fanout-type AMQP message
func PublishFanoutAMQPMessage(t *testing.T, host string, port int, queue, ownerID, eventType string, data map[string]interface{}, customHeaders map[string]string) {
	t.Helper()

	payload := map[string]interface{}{
		"owner_id":        ownerID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	if customHeaders != nil {
		payload["custom_headers"] = customHeaders
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	headers := map[string]interface{}{
		"x-convoy-message-type": "fanout",
	}

	PublishAMQPMessage(t, host, port, queue, body, headers)
}

// PublishBroadcastAMQPMessage publishes a broadcast-type AMQP message
func PublishBroadcastAMQPMessage(t *testing.T, host string, port int, queue, eventType string, data map[string]interface{}, customHeaders map[string]string) {
	t.Helper()

	payload := map[string]interface{}{
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	if customHeaders != nil {
		payload["custom_headers"] = customHeaders
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	headers := map[string]interface{}{
		"x-convoy-message-type": "broadcast",
	}

	PublishAMQPMessage(t, host, port, queue, body, headers)
}

// CreateSubscriptionWithFilter creates a subscription with advanced filtering
func CreateSubscriptionWithFilter(t *testing.T, db *postgres.Postgres, ctx context.Context,
	project *datastore.Project, endpoint *convoy.EndpointResponse, eventTypes []string,
	bodyFilter, headerFilter map[string]interface{}, function *string, sourceID *string) *datastore.Subscription {
	t.Helper()

	var nullFunc null.String
	if function != nil {
		nullFunc = null.StringFrom(*function)
	}

	subscription := &datastore.Subscription{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		Name:       fmt.Sprintf("subscription-%s", ulid.Make().String()),
		Type:       datastore.SubscriptionTypeAPI,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: eventTypes,
		},
		Function: nullFunc,
	}

	// Set source ID if provided (for broadcast messages with source filter)
	if sourceID != nil {
		subscription.SourceID = *sourceID
	}

	// Add advanced filters if provided
	if bodyFilter != nil || headerFilter != nil {
		subscription.FilterConfig.Filter = datastore.FilterSchema{
			Body:    bodyFilter,
			Headers: headerFilter,
		}
	}

	subRepo := subscriptions.New(log.NewLogger(os.Stdout), db)
	err := subRepo.CreateSubscription(ctx, project.UID, subscription)
	require.NoError(t, err)

	return subscription
}

// AssertEventCreated verifies that an event was created in the database
// Optional timeWindow parameter specifies the lookback window (defaults to 2 minutes if not provided)
func AssertEventCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventType string, timeWindow ...time.Duration) *datastore.Event {
	t.Helper()

	// Use default 2-minute window if not specified
	lookback := 2 * time.Minute
	if len(timeWindow) > 0 && timeWindow[0] > 0 {
		lookback = timeWindow[0]
	}

	// Use a short retry loop to account for async processing
	var event *datastore.Event
	var err error
	dbConn := db.GetDB()

	for i := 0; i < 15; i++ {
		// Query events directly from the events table
		query := `
			SELECT id, project_id, event_type, source_id, headers, raw, data,
				   created_at, updated_at, deleted_at, acknowledged_at,
				   idempotency_key, url_query_params, is_duplicate_event
			FROM convoy.events
			WHERE project_id = $1
			  AND event_type = $2
			  AND deleted_at IS NULL
			  AND created_at >= $3
			  AND created_at <= $4
			ORDER BY created_at DESC
			LIMIT 1
		`

		startTime := time.Now().Add(-lookback)
		endTime := time.Now().Add(1 * time.Minute)

		var e datastore.Event
		err = dbConn.QueryRowContext(ctx, query, projectID, eventType, startTime, endTime).Scan(
			&e.UID,
			&e.ProjectID,
			&e.EventType,
			&e.SourceID,
			&e.Headers,
			&e.Raw,
			&e.Data,
			&e.CreatedAt,
			&e.UpdatedAt,
			&e.DeletedAt,
			&e.AcknowledgedAt,
			&e.IdempotencyKey,
			&e.URLQueryParams,
			&e.IsDuplicateEvent,
		)

		if err == nil {
			t.Logf("✓ Found event: ID=%s, Type=%s, CreatedAt=%s (attempt %d)", e.UID, e.EventType, e.CreatedAt, i+1)
			event = &e
			break
		}

		if err != sql.ErrNoRows {
			t.Logf("ERROR querying events (attempt %d): %v", i+1, err)
		} else {
			t.Logf("No event found yet for project %s with type %s (attempt %d)", projectID, eventType, i+1)
		}

		time.Sleep(200 * time.Millisecond)
	}

	require.NoError(t, err)
	require.NotNil(t, event, "Event with type %s should have been created", eventType)
	require.Equal(t, eventType, string(event.EventType))

	return event
}

// AssertMultipleEventsCreated verifies that multiple events were created in the database
// and returns all matching events. The expectedCount parameter specifies how many events are expected.
func AssertMultipleEventsCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventType string, expectedCount int, timeWindow ...time.Duration) []*datastore.Event {
	t.Helper()

	// Use default 2-minute window if not specified
	lookback := 2 * time.Minute
	if len(timeWindow) > 0 && timeWindow[0] > 0 {
		lookback = timeWindow[0]
	}

	var events []*datastore.Event
	dbConn := db.GetDB()

	for i := 0; i < 30; i++ {
		query := `
			SELECT id, project_id, event_type, source_id, headers, raw, data,
				   created_at, updated_at, deleted_at, acknowledged_at,
				   idempotency_key, url_query_params, is_duplicate_event
			FROM convoy.events
			WHERE project_id = $1
			  AND event_type = $2
			  AND deleted_at IS NULL
			  AND created_at >= $3
			  AND created_at <= $4
			ORDER BY created_at ASC
		`

		startTime := time.Now().Add(-lookback)
		endTime := time.Now().Add(1 * time.Minute)

		rows, err := dbConn.QueryContext(ctx, query, projectID, eventType, startTime, endTime)
		if err != nil {
			t.Logf("ERROR querying events (attempt %d): %v", i+1, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if rows.Err() != nil {
			err = rows.Close()
			if err != nil {
				return nil
			}

			t.Logf("ERROR scanning events (attempt %d): %v", i+1, rows.Err())
			time.Sleep(500 * time.Millisecond)
			continue
		}

		events = nil // Reset for each attempt
		for rows.Next() {
			var e datastore.Event
			err := rows.Scan(
				&e.UID,
				&e.ProjectID,
				&e.EventType,
				&e.SourceID,
				&e.Headers,
				&e.Raw,
				&e.Data,
				&e.CreatedAt,
				&e.UpdatedAt,
				&e.DeletedAt,
				&e.AcknowledgedAt,
				&e.IdempotencyKey,
				&e.URLQueryParams,
				&e.IsDuplicateEvent,
			)
			if err != nil {
				rows.Close()
				t.Logf("ERROR scanning event (attempt %d): %v", i+1, err)
				break
			}
			events = append(events, &e)
		}
		rows.Close()

		if len(events) >= expectedCount {
			t.Logf("✓ Found %d events of type %s (attempt %d)", len(events), eventType, i+1)
			break
		}

		t.Logf("Found %d/%d events for project %s with type %s (attempt %d)", len(events), expectedCount, projectID, eventType, i+1)
		time.Sleep(500 * time.Millisecond)
	}

	require.GreaterOrEqual(t, len(events), expectedCount, "Expected at least %d events with type %s", expectedCount, eventType)
	return events
}

// AssertEventDeliveryCreated verifies that an event delivery was created
// Optional timeWindow parameter specifies the lookback window (defaults to 2 minutes if not provided)
func AssertEventDeliveryCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventID, endpointID string, timeWindow ...time.Duration) *datastore.EventDelivery {
	t.Helper()

	// Use default 2-minute window if not specified
	lookback := 2 * time.Minute
	if len(timeWindow) > 0 && timeWindow[0] > 0 {
		lookback = timeWindow[0]
	}

	// Use a short retry loop to account for async processing
	var eventDelivery *datastore.EventDelivery
	var err error
	dbConn := db.GetDB()

	for i := 0; i < 20; i++ {
		// Query event deliveries directly from the table
		query := `
			SELECT id, project_id, event_id, endpoint_id,
				   COALESCE(device_id, '') as device_id,
				   COALESCE(subscription_id, '') as subscription_id,
				   headers, attempts, status,
				   COALESCE(metadata::TEXT, '{}')::jsonb as metadata,
				   COALESCE(cli_metadata::TEXT, '{}')::jsonb as cli_metadata,
				   COALESCE(description, '') as description,
				   created_at, updated_at, deleted_at, acknowledged_at
			FROM convoy.event_deliveries
			WHERE project_id = $1
			  AND event_id = $2
			  AND endpoint_id = $3
			  AND deleted_at IS NULL
			  AND created_at >= $4
			  AND created_at <= $5
			ORDER BY created_at DESC
			LIMIT 1
		`

		startTime := time.Now().Add(-lookback)
		endTime := time.Now().Add(1 * time.Minute)

		var ed datastore.EventDelivery
		err = dbConn.QueryRowContext(ctx, query, projectID, eventID, endpointID, startTime, endTime).Scan(
			&ed.UID,
			&ed.ProjectID,
			&ed.EventID,
			&ed.EndpointID,
			&ed.DeviceID,
			&ed.SubscriptionID,
			&ed.Headers,
			&ed.DeliveryAttempts,
			&ed.Status,
			&ed.Metadata,
			&ed.CLIMetadata,
			&ed.Description,
			&ed.CreatedAt,
			&ed.UpdatedAt,
			&ed.DeletedAt,
			&ed.AcknowledgedAt,
		)

		if err == nil {
			t.Logf("✓ Found event delivery: ID=%s, EventID=%s, EndpointID=%s, Status=%s (attempt %d)", ed.UID, ed.EventID, ed.EndpointID, ed.Status, i+1)
			eventDelivery = &ed
			break
		}

		if err != sql.ErrNoRows {
			t.Logf("ERROR querying event deliveries (attempt %d): %v", i+1, err)
		} else {
			t.Logf("No event delivery found yet for event %s, endpoint %s (attempt %d)", eventID, endpointID, i+1)
		}

		time.Sleep(200 * time.Millisecond)
	}

	require.NoError(t, err)
	require.NotNil(t, eventDelivery, "Event delivery should have been created")
	require.Equal(t, eventID, eventDelivery.EventID)
	require.Equal(t, endpointID, eventDelivery.EndpointID)

	return eventDelivery
}

// AssertNoEventDeliveryCreated verifies that NO event delivery was created for a specific
// event and endpoint within a time window. This is used in negative test cases to verify
// that filtering logic (event types, body filters, headers) correctly prevents delivery creation.
//
// The function filters by both eventID AND endpointID to ensure test isolation when multiple
// endpoints exist in the same project.
//
// Optional timeWindow parameter specifies the lookback window (defaults to 5 minutes if not provided)
func AssertNoEventDeliveryCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventID, endpointID string, timeWindow ...time.Duration) {
	t.Helper()

	// Use default 5-minute window if not specified (larger window for negative tests)
	lookback := 5 * time.Minute
	if len(timeWindow) > 0 && timeWindow[0] > 0 {
		lookback = timeWindow[0]
	}

	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)

	// Wait a bit to ensure no delivery is created
	time.Sleep(2 * time.Second)

	now := time.Now()
	searchParams := datastore.SearchParams{
		CreatedAtStart: now.Add(-lookback).Unix(),
		CreatedAtEnd:   now.Add(1 * time.Minute).Unix(),
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}

	deliveries, _, err := eventDeliveryRepo.LoadEventDeliveriesPaged(
		ctx, projectID, []string{endpointID}, eventID, "",
		nil, searchParams, pageable, "", "", "",
	)
	require.NoError(t, err)
	require.Empty(t, deliveries, "No event delivery should have been created")
}

// AssertDeliveryAttemptCreated verifies that at least one delivery attempt was created
// for the specified event delivery. Returns the most recent delivery attempt for
// additional assertions (HTTP status, error details, etc.).
//
// This helper uses a retry loop (20 attempts, 200ms apart) to account for async
// delivery attempt creation after webhook delivery completes.
func AssertDeliveryAttemptCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, eventDeliveryID string) *datastore.DeliveryAttempt {
	t.Helper()

	attemptsService := delivery_attempts.New(nil, db)

	// Retry loop - delivery attempts may take time to be created
	var attempts []datastore.DeliveryAttempt
	var err error
	for i := 0; i < 20; i++ {
		attempts, err = attemptsService.FindDeliveryAttempts(ctx, eventDeliveryID)
		if err == nil && len(attempts) > 0 {
			t.Logf("✓ Found %d delivery attempt(s) for delivery %s (attempt %d)",
				len(attempts), eventDeliveryID, i+1)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	require.NoError(t, err, "Failed to query delivery attempts")
	require.NotEmpty(t, attempts, "At least one delivery attempt should have been created")

	// Return the most recent attempt
	return &attempts[len(attempts)-1]
}

// SQS Helper Functions

// CreateSQSSource creates an SQS source for E2E testing
func CreateSQSSource(t *testing.T, db *postgres.Postgres, ctx context.Context, project *datastore.Project, endpoint, queueName string, workers int, bodyFunction, headerFunction *string) *datastore.Source {
	t.Helper()
	source := &datastore.Source{
		UID:          ulid.Make().String(),
		ProjectID:    project.UID,
		MaskID:       ulid.Make().String(),
		Name:         fmt.Sprintf("sqs-source-%s", ulid.Make().String()),
		Type:         datastore.PubSubSource,
		Provider:     datastore.GithubSourceProvider,
		IsDisabled:   false,
		BodyFunction: bodyFunction,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
		HeaderFunction: headerFunction,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.SqsPubSub,
			Workers: workers,
			Sqs: &datastore.SQSPubSubConfig{
				AccessKeyID:   "test",
				SecretKey:     "test",
				DefaultRegion: "us-east-1",
				QueueName:     queueName,
				Endpoint:      endpoint,
			},
		},
	}

	sourceRepo := sources.New(log.NewLogger(io.Discard), db)
	err := sourceRepo.CreateSource(ctx, source)
	require.NoError(t, err)

	return source
}

// CreateSQSQueue creates an SQS queue in LocalStack and returns the queue URL
func CreateSQSQueue(t *testing.T, endpoint, queueName string) string {
	t.Helper()

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)

	// Create queue
	result, err := svc.CreateQueue(&sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	require.NoError(t, err)
	require.NotNil(t, result.QueueUrl)

	return *result.QueueUrl
}

// GetSQSQueueURL retrieves the queue URL for a given queue name
func GetSQSQueueURL(t *testing.T, endpoint, queueName string) string {
	t.Helper()

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)

	result, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	require.NoError(t, err)
	require.NotNil(t, result.QueueUrl)

	return *result.QueueUrl
}

// PublishSingleSQSMessage publishes a single-type message to SQS
func PublishSingleSQSMessage(t *testing.T, endpoint, queueURL, endpointID, eventType string, data map[string]interface{}) error {
	t.Helper()
	// Create message body
	messageBody := map[string]interface{}{
		"endpoint_id":     endpointID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)

	// Send message with attributes
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(bodyJSON)),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"x-convoy-message-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("single"),
			},
		},
	})

	return err
}

// PublishFanoutSQSMessage publishes a fanout-type message to SQS
func PublishFanoutSQSMessage(t *testing.T, endpoint, queueURL, ownerID, eventType string, data map[string]interface{}) error {
	t.Helper()
	// Create message body
	messageBody := map[string]interface{}{
		"owner_id":        ownerID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)

	// Send message with attributes
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(bodyJSON)),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"x-convoy-message-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("fanout"),
			},
		},
	})

	return err
}

// PublishBroadcastSQSMessage publishes a broadcast-type message to SQS
func PublishBroadcastSQSMessage(t *testing.T, endpoint, queueURL, eventType string, data map[string]interface{}) error {
	t.Helper()
	// Create message body
	messageBody := map[string]interface{}{
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)

	// Send message with attributes
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(bodyJSON)),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"x-convoy-message-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("broadcast"),
			},
		},
	})

	return err
}

// Kafka Helper Functions

// CreateKafkaSource creates a Kafka source for E2E testing
func CreateKafkaSource(t *testing.T, db *postgres.Postgres, ctx context.Context, project *datastore.Project,
	brokers []string, topic string, consumerGroupID string, workers int, bodyFunction, headerFunction *string) *datastore.Source {
	t.Helper()

	source := &datastore.Source{
		UID:          ulid.Make().String(),
		ProjectID:    project.UID,
		MaskID:       ulid.Make().String(),
		Name:         fmt.Sprintf("kafka-source-%s", ulid.Make().String()),
		Type:         datastore.PubSubSource,
		Provider:     datastore.GithubSourceProvider,
		IsDisabled:   false,
		BodyFunction: bodyFunction,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
		HeaderFunction: headerFunction,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.KafkaPubSub,
			Workers: workers,
			Kafka: &datastore.KafkaPubSubConfig{
				Brokers:         brokers,
				ConsumerGroupID: consumerGroupID,
				TopicName:       topic,
			},
		},
	}

	sourceRepo := sources.New(log.NewLogger(io.Discard), db)
	err := sourceRepo.CreateSource(ctx, source)
	require.NoError(t, err)

	return source
}

// CreateKafkaTopic creates a Kafka topic using kafka-go admin client
func CreateKafkaTopic(t *testing.T, broker, topic string, numPartitions, replicationFactor int) {
	t.Helper()

	conn, err := kafka.Dial("tcp", broker)
	require.NoError(t, err)
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err)

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	require.NoError(t, err)
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	})
	require.NoError(t, err)

	t.Logf("Created Kafka topic: %s", topic)
}

// PublishSingleKafkaMessage publishes a single-type message to Kafka
func PublishSingleKafkaMessage(t *testing.T, broker string, topic string,
	endpointID string, eventType string, data map[string]interface{}) {
	t.Helper()

	// Create message body
	messageBody := map[string]interface{}{
		"endpoint_id":     endpointID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Create Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{broker},
		Topic:   topic,
	})
	defer writer.Close()

	// Publish message with headers
	err = writer.WriteMessages(context.Background(), kafka.Message{
		Value: bodyJSON,
		Headers: []kafka.Header{
			{
				Key:   "x-convoy-message-type",
				Value: []byte("single"),
			},
		},
	})
	require.NoError(t, err)

	t.Logf("Published single Kafka message to topic %s", topic)
}

// PublishFanoutKafkaMessage publishes a fanout-type message to Kafka
func PublishFanoutKafkaMessage(t *testing.T, broker string, topic string,
	ownerID string, eventType string, data map[string]interface{}) {
	t.Helper()

	// Create message body
	messageBody := map[string]interface{}{
		"owner_id":        ownerID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Create Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{broker},
		Topic:   topic,
	})
	defer writer.Close()

	// Publish message with headers
	err = writer.WriteMessages(context.Background(), kafka.Message{
		Value: bodyJSON,
		Headers: []kafka.Header{
			{
				Key:   "x-convoy-message-type",
				Value: []byte("fanout"),
			},
		},
	})
	require.NoError(t, err)

	t.Logf("Published fanout Kafka message to topic %s", topic)
}

// PublishBroadcastKafkaMessage publishes a broadcast-type message to Kafka
func PublishBroadcastKafkaMessage(t *testing.T, broker string, topic string,
	eventType string, data map[string]interface{}) {
	t.Helper()

	// Create message body
	messageBody := map[string]interface{}{
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Create Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{broker},
		Topic:   topic,
	})
	defer writer.Close()

	// Publish message with headers
	err = writer.WriteMessages(context.Background(), kafka.Message{
		Value: bodyJSON,
		Headers: []kafka.Header{
			{
				Key:   "x-convoy-message-type",
				Value: []byte("broadcast"),
			},
		},
	})
	require.NoError(t, err)

	t.Logf("Published broadcast Kafka message to topic %s", topic)
}

// Google Pub/Sub Helper Functions

// CreateGooglePubSubSource creates a Google Pub/Sub source for E2E testing
func CreateGooglePubSubSource(t *testing.T, db *postgres.Postgres, ctx context.Context, project *datastore.Project,
	projectID string, subscriptionID string, workers int, bodyFunction, headerFunction *string) *datastore.Source {
	t.Helper()

	source := &datastore.Source{
		UID:          ulid.Make().String(),
		ProjectID:    project.UID,
		MaskID:       ulid.Make().String(),
		Name:         fmt.Sprintf("pubsub-source-%s", ulid.Make().String()),
		Type:         datastore.PubSubSource,
		Provider:     datastore.GithubSourceProvider,
		IsDisabled:   false,
		BodyFunction: bodyFunction,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
		HeaderFunction: headerFunction,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.GooglePubSub,
			Workers: workers,
			Google: &datastore.GooglePubSubConfig{
				ProjectID:      projectID,
				SubscriptionID: subscriptionID,
				// Valid service account JSON for emulator
				// When PUBSUB_EMULATOR_HOST is set, authentication is bypassed but the JSON must be structurally valid
				ServiceAccount: []byte(`{
					"type": "service_account",
					"project_id": "` + projectID + `",
					"private_key_id": "test-key-id",
					"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCnHlJP11mvWMEM\nh7W4zSv3+9zhM+e+u71TzjogbJulwbucjWPznod8qhoaN6M8QWX6xSSkMqsfGjSm\nwbFOY9DEy9vIYq+xZkBOTUebE71pfhtz3/LNMH6dFLIA+6YuFFVoMiHBJk8YB81l\nbGvOb9UXbBhQKL9uZY6EDe2VeyRc6VyNNefG0RfP/jSUl0DNIfyO0fKNaRZ+vYMD\nChPIGP+tQqHpV8leMGmLuzpSKz16AxacYEI/jPWIe7VGc4dZ4fXWcfxcvsXMWSvJ\nA/VB5NUyVgcncrl7LvOOgnMKqG7UAHLWbHOMy3Jv3M/JX1LAebO93gRJZwPov9UN\n6Q/WNIErAgMBAAECggEADE4Rvno7Sstsr3kAmuJUhfZgDZ7uRd958cVCB2gnz70j\njMPmY6Y9EKNPt7V4CfRAx4WjjImEiw45aTviy8RSt2LRRIBrslK2km1jQ9pgvHdC\nGzaWoKAc+oDvGF5vHn51yW3DiX7CHSFZ8MlaaMFYPdjCM4jEi6LjqvqBj1uZUlPn\n6YNxJZ5zbKOaQLs9zyODFIos51qvl1VF6IiYmuhMQPCNf4gqHcoQB8wlli9v2LX0\na1ZUC/iO4ZS8qU5KkNXJNRVM7Jm7ozA6hHpZy+vAmjMmHR80DqocQy+gc8h+F8ZE\n08/4odstGZsMKX1k/NeakMKM/bXqS2WQirRgPBjOzQKBgQDdyCpzetMrkuBa+8kJ\nQYf14VrKnOyyXNLiOEfOs3jye1p0BYiefuq9+1z7QA9PgXggoLxYwLM2vRMMzBSa\nhbolMK1jRn3t7jJ5B3cWZcMx9yfj5u9XHcpDGmjNfSoO3AhmVzrSjpOw4J6+S3lU\nWeDOyOet1HTWaMUKBiNHPGrTjQKBgQDA5xXDqJ1XGzBRUQAprb/yQAGz953Xz227\naFkdSrCYVh/3WTQd2bgBdjRQuCHsNlu89rF/2qaO9hUdnQrfMBM6kmCRqjtrWuir\njn7FU+R+mgXhqn5II0sEJy5vf32xGMMZF58ahEM3sLXKliPTk14r10YWW1O9OfWL\nU+fmIBXdlwKBgB09BluzFaPo+SsFhrtxqDsCOrX7ejkJg8PPJ6hYgNl26bXiBODg\nWpIxUVDOYTZaGzwx9KK+xOGyi5BkV1MHzkKY6ELuSCvV+1F5annJcLJloxyolWUm\nyEOQd8Cff6v11iWn2lln8pCfDE6KJLS6JKkeU2zXVY/uwAtSQ9RgYrUBAoGADW2I\nlFAec7vOxzpOOph/rgtKkw5/jFBCITOIUIOse04zd3JcMF/BcUibJ6tJoTm/dQ3v\nGSlNQtJace9GnHaqP/+EfV9ON5DidV678FyAoVdzZVwK4laimC1qDBTh2PwSSKLe\nTmg6jZvda7a707SEb6TSmifNUnTAZOx4TgqZuw0CgYEAhK/hUC/XB9slvcfdcjIl\nnL9177YOsze5d8H1c+X4KbnYCtMHqVUSBqzPE7mEBnYcYnPKMbaJSZEFWWSs03VJ\nzyWW47RpUlM7mF5TQknf1Wrztm+1vGl+/WqGfBWtcI+FmsADI8eNH5W3Zo1lsPx1\n62HDtYD7HVNKoJT0JMwmpDI=\n-----END PRIVATE KEY-----\n"",
					"client_email": "test@convoy-test-project.iam.gserviceaccount.com",
					"client_id": "123456789",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth",
					"token_uri": "https://oauth2.googleapis.com/token",
					"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs"
				}`),
			},
		},
	}

	sourceRepo := sources.New(log.NewLogger(io.Discard), db)
	err := sourceRepo.CreateSource(ctx, source)
	require.NoError(t, err)

	return source
}

// CreateGooglePubSubTopicAndSubscription creates a Pub/Sub topic and subscription using the emulator
func CreateGooglePubSubTopicAndSubscription(t *testing.T, emulatorHost, projectID, topicID, subscriptionID string) {
	t.Helper()

	ctx := context.Background()

	// Set emulator host environment variable only if not already set
	// This preserves the global setting from TestMain
	existingHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if existingHost == "" {
		os.Setenv("PUBSUB_EMULATOR_HOST", emulatorHost)
		defer os.Unsetenv("PUBSUB_EMULATOR_HOST")
	}

	// Create client
	client, err := pubsub.NewClient(ctx, projectID)
	require.NoError(t, err)
	defer client.Close()

	// Create topic
	topic, err := client.CreateTopic(ctx, topicID)
	require.NoError(t, err)

	t.Logf("Created Pub/Sub topic: %s", topicID)

	// Create subscription
	_, err = client.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
		Topic: topic,
	})
	require.NoError(t, err)

	t.Logf("Created Pub/Sub subscription: %s", subscriptionID)
}

// PublishSingleGooglePubSubMessage publishes a single-type message to Google Pub/Sub
func PublishSingleGooglePubSubMessage(t *testing.T, emulatorHost string, projectID string, topicID string,
	endpointID string, eventType string, data map[string]interface{}) {
	t.Helper()

	ctx := context.Background()

	// Set emulator host only if not already set
	existingHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if existingHost == "" {
		os.Setenv("PUBSUB_EMULATOR_HOST", emulatorHost)
		defer os.Unsetenv("PUBSUB_EMULATOR_HOST")
	}

	// Create client
	client, err := pubsub.NewClient(ctx, projectID)
	require.NoError(t, err)
	defer client.Close()

	// Create message body
	messageBody := map[string]interface{}{
		"endpoint_id":     endpointID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Get topic
	topic := client.Topic(topicID)

	// Publish message with attributes
	result := topic.Publish(ctx, &pubsub.Message{
		Data: bodyJSON,
		Attributes: map[string]string{
			"x-convoy-message-type": "single",
		},
	})

	// Wait for publish to complete
	_, err = result.Get(ctx)
	require.NoError(t, err)

	t.Logf("Published single Pub/Sub message to topic %s", topicID)
}

// PublishFanoutGooglePubSubMessage publishes a fanout-type message to Google Pub/Sub
func PublishFanoutGooglePubSubMessage(t *testing.T, emulatorHost string, projectID string, topicID string,
	ownerID string, eventType string, data map[string]interface{}) {
	t.Helper()

	ctx := context.Background()

	// Set emulator host only if not already set
	existingHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if existingHost == "" {
		os.Setenv("PUBSUB_EMULATOR_HOST", emulatorHost)
		defer os.Unsetenv("PUBSUB_EMULATOR_HOST")
	}

	// Create client
	client, err := pubsub.NewClient(ctx, projectID)
	require.NoError(t, err)
	defer client.Close()

	// Create message body
	messageBody := map[string]interface{}{
		"owner_id":        ownerID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Get topic
	topic := client.Topic(topicID)

	// Publish message with attributes
	result := topic.Publish(ctx, &pubsub.Message{
		Data: bodyJSON,
		Attributes: map[string]string{
			"x-convoy-message-type": "fanout",
		},
	})

	// Wait for publish to complete
	_, err = result.Get(ctx)
	require.NoError(t, err)

	t.Logf("Published fanout Pub/Sub message to topic %s", topicID)
}

// PublishBroadcastGooglePubSubMessage publishes a broadcast-type message to Google Pub/Sub
func PublishBroadcastGooglePubSubMessage(t *testing.T, emulatorHost string, projectID string, topicID string,
	eventType string, data map[string]interface{}) {
	t.Helper()

	ctx := context.Background()

	// Set emulator host only if not already set
	existingHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if existingHost == "" {
		os.Setenv("PUBSUB_EMULATOR_HOST", emulatorHost)
		defer os.Unsetenv("PUBSUB_EMULATOR_HOST")
	}

	// Create client
	client, err := pubsub.NewClient(ctx, projectID)
	require.NoError(t, err)
	defer client.Close()

	// Create message body
	messageBody := map[string]interface{}{
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	bodyJSON, err := json.Marshal(messageBody)
	require.NoError(t, err)

	// Get topic
	topic := client.Topic(topicID)

	// Publish message with attributes
	result := topic.Publish(ctx, &pubsub.Message{
		Data: bodyJSON,
		Attributes: map[string]string{
			"x-convoy-message-type": "broadcast",
		},
	})

	// Wait for publish to complete
	_, err = result.Get(ctx)
	require.NoError(t, err)

	t.Logf("Published broadcast Pub/Sub message to topic %s", topicID)
}

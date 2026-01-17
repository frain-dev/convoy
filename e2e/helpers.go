package e2e

import (
	"context"
	"database/sql"
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
	"gopkg.in/guregu/null.v4"

	convoy "github.com/frain-dev/convoy-go/v2"
	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
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

	sourceRepo := postgres.NewSourceRepo(db)
	err := sourceRepo.CreateSource(ctx, source)
	require.NoError(t, err)

	return source
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

	subRepo := postgres.NewSubscriptionRepo(db)
	err := subRepo.CreateSubscription(ctx, project.UID, subscription)
	require.NoError(t, err)

	return subscription
}

// AssertEventCreated verifies that an event was created in the database
func AssertEventCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventType string) *datastore.Event {
	t.Helper()

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

		startTime := time.Now().Add(-2 * time.Minute)
		endTime := time.Now().Add(2 * time.Minute)

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

// AssertEventDeliveryCreated verifies that an event delivery was created
func AssertEventDeliveryCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventID, endpointID string) *datastore.EventDelivery {
	t.Helper()

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
				   COALESCE(metadata::text, '{}')::jsonb as metadata,
				   COALESCE(cli_metadata::text, '{}')::jsonb as cli_metadata,
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

		startTime := time.Now().Add(-2 * time.Minute)
		endTime := time.Now().Add(2 * time.Minute)

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

// AssertNoEventDeliveryCreated verifies that NO event delivery was created (for negative tests)
func AssertNoEventDeliveryCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventID string) {
	t.Helper()

	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)

	// Wait a bit to ensure no delivery is created
	time.Sleep(2 * time.Second)

	searchParams := datastore.SearchParams{
		CreatedAtStart: time.Now().Add(-1 * time.Minute).Unix(),
		CreatedAtEnd:   time.Now().Add(1 * time.Minute).Unix(),
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}

	deliveries, _, err := eventDeliveryRepo.LoadEventDeliveriesPaged(
		ctx, projectID, nil, eventID, "",
		nil, searchParams, pageable, "", "", "",
	)
	require.NoError(t, err)
	require.Empty(t, deliveries, "No event delivery should have been created")
}

// AssertDeliveryAttemptCreated verifies that a delivery attempt was created
func AssertDeliveryAttemptCreated(t *testing.T, db *postgres.Postgres, ctx context.Context, projectID, eventDeliveryID string) {
	t.Helper()

	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)

	// Use a short retry loop - fetch the event delivery and check if it has attempts
	var delivery *datastore.EventDelivery
	var err error
	for i := 0; i < 20; i++ {
		delivery, err = eventDeliveryRepo.FindEventDeliveryByID(ctx, projectID, eventDeliveryID)
		if err == nil {
			t.Logf("Event delivery loaded: ID=%s, Status=%s, Attempts=%d (attempt %d)", delivery.UID, delivery.Status, len(delivery.DeliveryAttempts), i+1)
			if len(delivery.DeliveryAttempts) > 0 {
				t.Logf("✓ Found delivery attempts: %d", len(delivery.DeliveryAttempts))
				break
			}
		} else {
			t.Logf("ERROR loading event delivery (attempt %d): %v", i+1, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	require.NoError(t, err)
	require.NotNil(t, delivery, "Event delivery should exist")
	require.NotEmpty(t, delivery.DeliveryAttempts, "At least one delivery attempt should have been created")
}

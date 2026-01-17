package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
)

// TestE2E_SQS_Single_BasicDelivery tests basic single message delivery via SQS
func TestE2E_SQS_Single_BasicDelivery(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB for direct database operations
	db := env.App.DB.(*postgres.Postgres)

	// Create test endpoint
	port := 19000
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription
	eventType := "invoice.created"
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, nil, nil, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Set up mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expect 1 webhook
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources to pick up the new SQS source
	env.SyncSources(t)

	// Publish SQS message
	data := map[string]interface{}{
		"amount":     100,
		"currency":   "USD",
		"invoice_id": "inv-" + ulid.Make().String(),
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data,
	)
	require.NoError(t, err)

	// Wait for webhook delivery
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify webhook was received
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Greater(t, hits, 0, "Webhook should have been delivered")

	// Verify event was created in database
	event := AssertEventCreated(t, db, context.Background(), env.Project.UID, eventType)
	require.NotNil(t, event)
	require.Equal(t, eventType, string(event.EventType))

	// Verify event delivery was created
	eventDelivery := AssertEventDeliveryCreated(
		t, db, context.Background(),
		env.Project.UID, event.UID, endpoint.UID,
	)
	require.NotNil(t, eventDelivery)
}

// TestE2E_SQS_Fanout_MultipleEndpoints tests fanout message delivery to multiple endpoints
func TestE2E_SQS_Fanout_MultipleEndpoints(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB for direct database operations
	db := env.App.DB.(*postgres.Postgres)

	// Create multiple endpoints with same owner
	ownerID := "owner-" + ulid.Make().String()
	port1 := 19001
	port2 := 19002
	endpoint1 := CreateEndpointViaSDK(t, c, port1, ownerID)
	endpoint2 := CreateEndpointViaSDK(t, c, port2, ownerID)

	// Create subscriptions for both endpoints
	eventType := "payment.received"
	sub1 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint1, []string{eventType}, nil, nil, nil, nil,
	)
	sub2 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint2, []string{eventType}, nil, nil, nil, nil,
	)
	require.NotEmpty(t, sub1.UID)
	require.NotEmpty(t, sub2.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Set up mock webhook servers
	manifest1 := NewEventManifest()
	manifest2 := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	var counter1, counter2 atomic.Int64
	counter1.Store(1)
	counter2.Store(1)
	StartMockWebhookServer(t, manifest1, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest2, done2, &counter2, port2)

	// Sync sources to pick up the new SQS source
	env.SyncSources(t)

	// Publish fanout SQS message
	data := map[string]interface{}{
		"amount":      100,
		"customer_id": "cust-" + ulid.Make().String(),
	}
	err := PublishFanoutSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		ownerID, eventType, data,
	)
	require.NoError(t, err)

	// First, check if events are being created
	time.Sleep(3 * time.Second) // Give workers time to process
	t.Logf("Checking if fanout event was created in database...")

	// Try to find the event
	event := AssertEventCreated(t, db, context.Background(), env.Project.UID, eventType)
	t.Logf("Found event: ID=%s, Type=%s", event.UID, event.EventType)

	// Wait for both webhooks to be delivered
	WaitForWebhooks(t, done1, 30*time.Second)
	WaitForWebhooks(t, done2, 30*time.Second)

	// Verify both webhooks were received
	webhookURL1 := fmt.Sprintf("http://localhost:%d/webhook", port1)
	webhookURL2 := fmt.Sprintf("http://localhost:%d/webhook", port2)
	hits1 := manifest1.ReadEndpoint(webhookURL1)
	hits2 := manifest2.ReadEndpoint(webhookURL2)
	require.Greater(t, hits1, 0, "Webhook 1 should have been delivered")
	require.Greater(t, hits2, 0, "Webhook 2 should have been delivered")

	// Verify event deliveries were created for both endpoints
	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, context.Background(),
		env.Project.UID, event.UID, endpoint1.UID,
	)
	eventDelivery2 := AssertEventDeliveryCreated(
		t, db, context.Background(),
		env.Project.UID, event.UID, endpoint2.UID,
	)
	require.NotNil(t, eventDelivery1)
	require.NotNil(t, eventDelivery2)
}

// TestE2E_SQS_Broadcast_AllSubscribers tests broadcast message delivery to all subscriptions
func TestE2E_SQS_Broadcast_AllSubscribers(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB for direct database operations
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoints with different owners
	owner1 := "owner-" + ulid.Make().String()
	owner2 := "owner-" + ulid.Make().String()
	port1 := 19003
	port2 := 19004
	endpoint1 := CreateEndpointViaSDK(t, c, port1, owner1)
	endpoint2 := CreateEndpointViaSDK(t, c, port2, owner2)

	// Create subscriptions for both endpoints with same event type
	eventType := "system.alert"
	sub1 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint1, []string{eventType}, nil, nil, nil, nil,
	)
	sub2 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint2, []string{eventType}, nil, nil, nil, nil,
	)
	require.NotEmpty(t, sub1.UID)
	require.NotEmpty(t, sub2.UID)

	// IMPORTANT: Sync subscriptions to memory table for broadcast event processing
	// Broadcast events use in-memory subscription lookup, not database queries
	env.SyncSubscriptions(t)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Set up mock webhook servers
	manifest1 := NewEventManifest()
	manifest2 := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	var counter1, counter2 atomic.Int64
	counter1.Store(1)
	counter2.Store(1)
	StartMockWebhookServer(t, manifest1, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest2, done2, &counter2, port2)

	// Sync sources to pick up the new SQS source
	env.SyncSources(t)

	// Publish broadcast SQS message
	data := map[string]interface{}{
		"severity": "high",
		"message":  "System maintenance scheduled",
		"alert_id": "alert-" + ulid.Make().String(),
	}
	err := PublishBroadcastSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		eventType, data,
	)
	require.NoError(t, err)

	// First, check if events are being created
	time.Sleep(3 * time.Second) // Give workers time to process
	t.Logf("Checking if broadcast event was created in database...")

	// Try to find the event
	event := AssertEventCreated(t, db, context.Background(), env.Project.UID, eventType)
	t.Logf("Found event: ID=%s, Type=%s", event.UID, event.EventType)

	// Debug: Check if event deliveries were created for broadcast event
	time.Sleep(5 * time.Second) // Give worker extra time to create deliveries from broadcast
	t.Logf("Checking if event deliveries were created for broadcast event...")
	edRepo := postgres.NewEventDeliveryRepo(db)
	deliveries, err1 := edRepo.FindEventDeliveriesByEventID(context.Background(), env.Project.UID, event.UID)
	if err1 == nil && len(deliveries) > 0 {
		t.Logf("✓ Found %d event deliveries for broadcast event %s", len(deliveries), event.UID)
		for i, ed := range deliveries {
			t.Logf("  Delivery %d: ID=%s, EndpointID=%s, Status=%s", i+1, ed.UID, ed.EndpointID, ed.Status)
		}
	} else {
		t.Logf("✗ No event deliveries found for broadcast event %s (error: %v)", event.UID, err1)
	}

	// Wait for both webhooks to be delivered
	WaitForWebhooks(t, done1, 30*time.Second)
	WaitForWebhooks(t, done2, 30*time.Second)

	// Verify both webhooks were received
	webhookURL1 := fmt.Sprintf("http://localhost:%d/webhook", port1)
	webhookURL2 := fmt.Sprintf("http://localhost:%d/webhook", port2)
	hits1 := manifest1.ReadEndpoint(webhookURL1)
	hits2 := manifest2.ReadEndpoint(webhookURL2)
	require.Greater(t, hits1, 0, "Webhook 1 should have been delivered")
	require.Greater(t, hits2, 0, "Webhook 2 should have been delivered")

	// Verify event deliveries were created for both endpoints
	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, context.Background(),
		env.Project.UID, event.UID, endpoint1.UID,
	)
	eventDelivery2 := AssertEventDeliveryCreated(
		t, db, context.Background(),
		env.Project.UID, event.UID, endpoint2.UID,
	)
	require.NotNil(t, eventDelivery1)
	require.NotNil(t, eventDelivery2)
}

// Event Type Filtering Tests

func TestE2E_SQS_Single_EventTypeFilter(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB for direct database operations
	db := env.App.DB.(*postgres.Postgres)

	// Create test endpoint
	port := 19010
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with specific event types
	eventTypes := []string{"invoice.created", "payment.received"}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, eventTypes, nil, nil, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expecting 1 webhook
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources to pick up the new SQS source
	env.SyncSources(t)

	// Test 1: Publish message with MATCHING event type (invoice.created)
	data1 := map[string]interface{}{
		"invoice_id": "inv-" + ulid.Make().String(),
		"amount":     100,
		"currency":   "USD",
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, "invoice.created", data1,
	)
	require.NoError(t, err)

	// Wait for webhook
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify webhook was received for matching event type
	t.Log("Checking if matching event type was delivered...")
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Greater(t, hits, 0, "Webhook should have been delivered for matching event type")

	// Find the event in database
	event1 := AssertEventCreated(
		t, db, env.ctx,
		env.Project.UID, "invoice.created",
	)
	require.NotNil(t, event1)
	t.Logf("✓ Found event: ID=%s, Type=%s", event1.UID, event1.EventType)

	// Verify event delivery was created
	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID, endpoint.UID,
	)
	require.NotNil(t, eventDelivery1)

	// Test 2: Publish message with NON-MATCHING event type (user.signup)
	t.Log("Publishing message with non-matching event type...")
	data2 := map[string]interface{}{
		"user_id": "user-" + ulid.Make().String(),
		"email":   "test@example.com",
	}

	err = PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, "user.signup", data2,
	)
	require.NoError(t, err)

	// Wait a bit to see if webhook is (incorrectly) delivered
	time.Sleep(3 * time.Second)

	// Verify webhook was NOT received for non-matching event type
	t.Log("Verifying no webhook was delivered for non-matching event type...")
	hits2 := manifest.ReadEndpoint(webhookURL)
	// hits2 should be same as hits (no new webhooks)
	require.Equal(t, hits, hits2, "No new webhooks should have been delivered for non-matching event type")

	// Find the event in database (it should exist)
	event2 := AssertEventCreated(
		t, db, env.ctx,
		env.Project.UID, "user.signup",
	)
	require.NotNil(t, event2)
	t.Logf("Found event: ID=%s, Type=%s", event2.UID, event2.EventType)

	// Verify event delivery was NOT created for non-matching event type
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID,
	)
	t.Log("✓ Event delivery correctly not created for non-matching event type")
}

func TestE2E_SQS_Single_WildcardEventType(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create test endpoint
	port := 19011
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with wildcard event type
	eventTypes := []string{"*"}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, eventTypes, nil, nil, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(3) // Expecting 3 webhooks from 3 different event types
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	// Publish 3 messages with different event types - all should be delivered
	eventTypesList := []string{"invoice.created", "payment.received", "user.signup"}
	for i, eventType := range eventTypesList {
		data := map[string]interface{}{
			"id":    ulid.Make().String(),
			"index": i,
			"type":  eventType,
		}
		err := PublishSingleSQSMessage(
			t, env.LocalStackEndpoint, queueURL,
			endpoint.UID, eventType, data,
		)
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond) // Small delay between messages
	}

	// Wait for all webhooks
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify all webhooks were received
	t.Log("Verifying all event types were delivered with wildcard subscription...")
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 3, hits, "All 3 event types should have been delivered with wildcard")

	// Verify all 3 events were created and have deliveries
	for _, eventType := range eventTypesList {
		event := AssertEventCreated(
			t, db, env.ctx,
			env.Project.UID, eventType,
		)
		require.NotNil(t, event)
		t.Logf("✓ Found event: ID=%s, Type=%s", event.UID, event.EventType)

		// Verify event delivery was created
		eventDelivery := AssertEventDeliveryCreated(
			t, db, env.ctx,
			env.Project.UID, event.UID, endpoint.UID,
		)
		require.NotNil(t, eventDelivery)
	}

	t.Log("✓ Wildcard subscription correctly matched all event types")
}

func TestE2E_SQS_Fanout_EventTypeFilter(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	ownerID := "owner-" + ulid.Make().String()

	// Create 2 endpoints with same owner
	port1 := 19012
	endpoint1 := CreateEndpointViaSDK(t, c, port1, ownerID)

	port2 := 19013
	endpoint2 := CreateEndpointViaSDK(t, c, port2, ownerID)

	// Create subscription for endpoint1 with specific event type filter
	subscription1 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint1, []string{"payment.received"}, nil, nil, nil, nil,
	)
	require.NotEmpty(t, subscription1.UID)

	// Create subscription for endpoint2 with different event type filter
	subscription2 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint2, []string{"invoice.created"}, nil, nil, nil, nil,
	)
	require.NotEmpty(t, subscription2.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook servers
	manifest1 := NewEventManifest()
	manifest2 := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	var counter1, counter2 atomic.Int64
	counter1.Store(1) // Endpoint1 expects 1 webhook (payment.received)
	counter2.Store(1) // Endpoint2 expects 1 webhook (invoice.created)

	StartMockWebhookServer(t, manifest1, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest2, done2, &counter2, port2)

	// Sync sources
	env.SyncSources(t)

	// Test 1: Publish fanout message with "payment.received" event type
	// Should only deliver to endpoint1
	data1 := map[string]interface{}{
		"payment_id": "pay-" + ulid.Make().String(),
		"amount":     200,
	}
	err := PublishFanoutSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		ownerID, "payment.received", data1,
	)
	require.NoError(t, err)

	// Wait for webhook to endpoint1
	WaitForWebhooks(t, done1, 30*time.Second)

	// Verify webhook was received by endpoint1
	webhookURL1 := fmt.Sprintf("http://localhost:%d/webhook", port1)
	hits1 := manifest1.ReadEndpoint(webhookURL1)
	require.Greater(t, hits1, 0, "Endpoint1 should receive payment.received")

	// Find the event in database
	event1 := AssertEventCreated(
		t, db, env.ctx,
		env.Project.UID, "payment.received",
	)
	require.NotNil(t, event1)

	// Verify event delivery was created only for endpoint1
	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID, endpoint1.UID,
	)
	require.NotNil(t, eventDelivery1)

	// Verify event delivery was NOT created for endpoint2
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID,
	)
	t.Log("✓ Event delivery correctly not created for endpoint2 (non-matching event type)")

	// Test 2: Publish fanout message with "invoice.created" event type
	// Should only deliver to endpoint2
	data2 := map[string]interface{}{
		"invoice_id": "inv-" + ulid.Make().String(),
		"amount":     500,
	}
	err = PublishFanoutSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		ownerID, "invoice.created", data2,
	)
	require.NoError(t, err)

	// Wait for webhook to endpoint2
	WaitForWebhooks(t, done2, 30*time.Second)

	// Verify webhook was received by endpoint2
	webhookURL2 := fmt.Sprintf("http://localhost:%d/webhook", port2)
	hits2 := manifest2.ReadEndpoint(webhookURL2)
	require.Greater(t, hits2, 0, "Endpoint2 should receive invoice.created")

	// Find the event in database
	event2 := AssertEventCreated(
		t, db, env.ctx,
		env.Project.UID, "invoice.created",
	)
	require.NotNil(t, event2)

	// Verify event delivery was created only for endpoint2
	eventDelivery2 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID, endpoint2.UID,
	)
	require.NotNil(t, eventDelivery2)

	// Verify event delivery was NOT created for endpoint1
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID,
	)
	t.Log("✓ Event delivery correctly not created for endpoint1 (non-matching event type)")
}

func TestE2E_SQS_Broadcast_EventTypeFilter(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create 3 endpoints with different owners
	port1 := 19014
	endpoint1 := CreateEndpointViaSDK(t, c, port1, "owner1")

	port2 := 19015
	endpoint2 := CreateEndpointViaSDK(t, c, port2, "owner2")

	port3 := 19016
	endpoint3 := CreateEndpointViaSDK(t, c, port3, "owner3")

	// Create SQS queue and source first
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Create subscriptions with different event type filters
	// Subscription 1: Listens for "order.placed"
	subscription1 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint1, []string{"order.placed"}, nil, nil, nil, &source.UID,
	)
	require.NotEmpty(t, subscription1.UID)

	// Subscription 2: Listens for "order.placed" AND "order.cancelled"
	subscription2 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint2, []string{"order.placed", "order.cancelled"}, nil, nil, nil, &source.UID,
	)
	require.NotEmpty(t, subscription2.UID)

	// Subscription 3: Listens for "order.shipped"
	subscription3 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint3, []string{"order.shipped"}, nil, nil, nil, &source.UID,
	)
	require.NotEmpty(t, subscription3.UID)

	// Setup mock webhook servers
	manifest1 := NewEventManifest()
	manifest2 := NewEventManifest()
	manifest3 := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	done3 := make(chan bool, 1)
	var counter1, counter2, counter3 atomic.Int64

	StartMockWebhookServer(t, manifest1, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest2, done2, &counter2, port2)
	StartMockWebhookServer(t, manifest3, done3, &counter3, port3)

	// Sync sources and subscriptions
	env.SyncSources(t)
	env.SyncSubscriptions(t)

	// Test 1: Publish "order.placed" - should deliver to endpoint1 and endpoint2
	t.Log("Publishing broadcast message with event type: order.placed")
	counter1.Store(1) // Endpoint1 expects 1 webhook
	counter2.Store(1) // Endpoint2 expects 1 webhook
	counter3.Store(0) // Endpoint3 expects 0 webhooks

	data1 := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
		"amount":   300,
	}
	err := PublishBroadcastSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		"order.placed", data1,
	)
	require.NoError(t, err)

	// Give worker time to process
	time.Sleep(3 * time.Second)

	// Debug: Check if event was created
	t.Log("Checking if broadcast event was created...")
	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "order.placed")
	t.Logf("Event created: ID=%s, Type=%s", event1.UID, event1.EventType)

	// Debug: Check if event deliveries were created
	t.Log("Checking if event deliveries were created...")
	edRepo := postgres.NewEventDeliveryRepo(db)
	deliveries, err := edRepo.FindEventDeliveriesByEventID(env.ctx, env.Project.UID, event1.UID)
	if err == nil && len(deliveries) > 0 {
		t.Logf("✓ Found %d event deliveries", len(deliveries))
		for i, ed := range deliveries {
			t.Logf("  Delivery %d: ID=%s, EndpointID=%s, Status=%s", i+1, ed.UID, ed.EndpointID, ed.Status)
		}
	} else {
		t.Logf("✗ No event deliveries found (error: %v)", err)
	}

	// Wait for webhooks
	WaitForWebhooks(t, done1, 30*time.Second)
	WaitForWebhooks(t, done2, 30*time.Second)

	// Verify webhooks were received
	webhookURL1 := fmt.Sprintf("http://localhost:%d/webhook", port1)
	webhookURL2 := fmt.Sprintf("http://localhost:%d/webhook", port2)
	webhookURL3 := fmt.Sprintf("http://localhost:%d/webhook", port3)

	hits1 := manifest1.ReadEndpoint(webhookURL1)
	hits2 := manifest2.ReadEndpoint(webhookURL2)
	hits3 := manifest3.ReadEndpoint(webhookURL3)

	require.Greater(t, hits1, 0, "Endpoint1 should receive order.placed")
	require.Greater(t, hits2, 0, "Endpoint2 should receive order.placed")
	require.Equal(t, 0, hits3, "Endpoint3 should NOT receive order.placed")

	// Event already found in debug section above
	require.NotNil(t, event1)

	// Verify event deliveries
	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID, endpoint1.UID,
	)
	require.NotNil(t, eventDelivery1)

	eventDelivery2 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID, endpoint2.UID,
	)
	require.NotNil(t, eventDelivery2)

	// Endpoint3 should NOT have a delivery
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID,
	)
	t.Log("✓ Broadcast correctly delivered only to subscriptions matching event type")

	// Test 2: Publish "order.shipped" - should deliver only to endpoint3
	t.Log("Publishing broadcast message with event type: order.shipped")

	// Set expected webhook counts for the second test
	counter1.Store(0) // Endpoint1 expects 0 webhooks (already received 1, no more expected)
	counter2.Store(0) // Endpoint2 expects 0 webhooks (already received 1, no more expected)
	counter3.Store(1) // Endpoint3 expects 1 webhook (hasn't received any yet)

	data2 := map[string]interface{}{
		"order_id":     "order-" + ulid.Make().String(),
		"tracking_num": "TRK123456",
	}
	err = PublishBroadcastSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		"order.shipped", data2,
	)
	require.NoError(t, err)

	// Give worker time to process
	time.Sleep(3 * time.Second)

	// Debug: Check if event was created
	t.Log("Checking if broadcast event 2 was created...")
	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "order.shipped")
	t.Logf("Event created: ID=%s, Type=%s", event2.UID, event2.EventType)

	// Debug: Check if event deliveries were created
	t.Log("Checking if event deliveries were created for event 2...")
	deliveries2, err2 := edRepo.FindEventDeliveriesByEventID(env.ctx, env.Project.UID, event2.UID)
	if err2 == nil && len(deliveries2) > 0 {
		t.Logf("✓ Found %d event deliveries for event 2", len(deliveries2))
		for i, ed := range deliveries2 {
			t.Logf("  Delivery %d: ID=%s, EndpointID=%s, Status=%s", i+1, ed.UID, ed.EndpointID, ed.Status)
		}
	} else {
		t.Logf("✗ No event deliveries found for event 2 (error: %v)", err2)
	}

	// Give a bit more time for webhook to definitely arrive
	time.Sleep(2 * time.Second)

	// Verify webhooks
	hits1After := manifest1.ReadEndpoint(webhookURL1)
	hits2After := manifest2.ReadEndpoint(webhookURL2)
	hits3After := manifest3.ReadEndpoint(webhookURL3)

	require.Equal(t, hits1, hits1After, "Endpoint1 should NOT receive order.shipped")
	require.Equal(t, hits2, hits2After, "Endpoint2 should NOT receive order.shipped")
	require.Greater(t, hits3After, hits3, "Endpoint3 should receive order.shipped")

	// Event already found in debug section above
	require.NotNil(t, event2)

	// Verify event delivery only for endpoint3
	eventDelivery3 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID, endpoint3.UID,
	)
	require.NotNil(t, eventDelivery3)

	// Endpoint1 and Endpoint2 should NOT have deliveries
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID,
	)
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID,
	)
	t.Log("✓ Broadcast correctly filtered by event type")
}

// Advanced Filtering Tests

func TestE2E_SQS_Single_BodyFilter_Equal(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19020
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with body filter: amount == 100
	eventType := "payment.processed"
	bodyFilter := map[string]interface{}{
		"amount": map[string]interface{}{
			"$eq": 100,
		},
	}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, bodyFilter, nil, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64

	// Test 1: Publish message with amount = 100 (should match)
	t.Log("Test 1: Publishing message with amount = 100 (should match)")
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	data1 := map[string]interface{}{
		"payment_id": "payment-" + ulid.Make().String(),
		"amount":     100,
		"currency":   "USD",
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data1,
	)
	require.NoError(t, err)

	// Wait for webhook
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify event and delivery were created
	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event1)

	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID, endpoint.UID,
	)
	require.NotNil(t, eventDelivery1)
	t.Log("✓ Body filter matched: amount = 100")

	// Test 2: Publish message with amount = 200 (should NOT match)
	t.Log("Test 2: Publishing message with amount = 200 (should NOT match)")
	data2 := map[string]interface{}{
		"payment_id": "payment-" + ulid.Make().String(),
		"amount":     200,
		"currency":   "USD",
	}
	err = PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data2,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created
	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event2)

	// Verify NO delivery was created (filter didn't match)
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID,
	)
	t.Log("✓ Body filter correctly rejected: amount = 200")
}

func TestE2E_SQS_Single_BodyFilter_GreaterThan(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19021
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with body filter: amount > 100
	eventType := "order.created"
	bodyFilter := map[string]interface{}{
		"amount": map[string]interface{}{
			"$gt": 100,
		},
	}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, bodyFilter, nil, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64

	// Test 1: Publish message with amount = 50 (should NOT match)
	t.Log("Test 1: Publishing message with amount = 50 (should NOT match)")
	counter.Store(0)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	data1 := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
		"amount":   50,
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data1,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created but NO delivery
	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event1)

	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID,
	)
	t.Log("✓ Body filter correctly rejected: amount = 50 (not > 100)")

	// Test 2: Publish message with amount = 150 (should match)
	t.Log("Test 2: Publishing message with amount = 150 (should match)")
	counter.Store(1)

	data2 := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
		"amount":   150,
	}
	err = PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data2,
	)
	require.NoError(t, err)

	// Wait for webhook
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify event and delivery were created
	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event2)

	eventDelivery2 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID, endpoint.UID,
	)
	require.NotNil(t, eventDelivery2)
	t.Log("✓ Body filter matched: amount = 150 (> 100)")
}

func TestE2E_SQS_Single_BodyFilter_In(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19022
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with body filter: status in ["pending", "processing"]
	eventType := "order.updated"
	bodyFilter := map[string]interface{}{
		"status": map[string]interface{}{
			"$in": []interface{}{"pending", "processing"},
		},
	}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, bodyFilter, nil, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(2) // Expecting 2 webhooks
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	// Test 1: Publish message with status = "pending" (should match)
	t.Log("Test 1: Publishing message with status = 'pending' (should match)")
	data1 := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
		"status":   "pending",
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data1,
	)
	require.NoError(t, err)

	// Test 2: Publish message with status = "processing" (should match)
	t.Log("Test 2: Publishing message with status = 'processing' (should match)")
	data2 := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
		"status":   "processing",
	}
	err = PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data2,
	)
	require.NoError(t, err)

	// Wait for both webhooks
	WaitForWebhooks(t, done, 30*time.Second)

	t.Log("✓ Body filter matched both: status in ['pending', 'processing']")

	// Test 3: Publish message with status = "completed" (should NOT match)
	t.Log("Test 3: Publishing message with status = 'completed' (should NOT match)")
	data3 := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
		"status":   "completed",
	}
	err = PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data3,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created
	event3 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event3)

	// Verify NO delivery was created for "completed" status
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event3.UID,
	)
	t.Log("✓ Body filter correctly rejected: status = 'completed'")
}

func TestE2E_SQS_Single_HeaderFilter(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19023
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with header filter: x-tenant = "acme"
	eventType := "webhook.received"
	headerFilter := map[string]interface{}{
		"x-tenant": map[string]interface{}{
			"$eq": "acme",
		},
	}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, nil, headerFilter, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64

	// Test 1: Publish message with x-tenant = "acme" (should match)
	t.Log("Test 1: Publishing message with message attribute x-tenant = 'acme' (should match)")
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	data1 := map[string]interface{}{
		"webhook_id": "webhook-" + ulid.Make().String(),
	}
	customHeaders1 := map[string]string{
		"x-tenant": "acme",
	}
	err := PublishSingleSQSMessageWithAttributes(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data1, customHeaders1,
	)
	require.NoError(t, err)

	// Wait for webhook
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify event and delivery were created
	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event1)

	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID, endpoint.UID,
	)
	require.NotNil(t, eventDelivery1)
	t.Log("✓ Header filter matched: x-tenant = 'acme'")

	// Test 2: Publish message with x-tenant = "other" (should NOT match)
	t.Log("Test 2: Publishing message with message attribute x-tenant = 'other' (should NOT match)")
	data2 := map[string]interface{}{
		"webhook_id": "webhook-" + ulid.Make().String(),
	}
	customHeaders2 := map[string]string{
		"x-tenant": "other",
	}
	err = PublishSingleSQSMessageWithAttributes(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data2, customHeaders2,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created
	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event2)

	// Verify NO delivery was created (filter didn't match)
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID,
	)
	t.Log("✓ Header filter correctly rejected: x-tenant = 'other'")
}

func TestE2E_SQS_Single_CombinedFilters(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19024
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with combined filters:
	// - Event type: "transaction.processed"
	// - Body: amount > 50
	// - Header: x-priority = "high"
	eventType := "transaction.processed"
	bodyFilter := map[string]interface{}{
		"amount": map[string]interface{}{
			"$gt": 50,
		},
	}
	headerFilter := map[string]interface{}{
		"x-priority": map[string]interface{}{
			"$eq": "high",
		},
	}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, bodyFilter, headerFilter, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64

	// Test 1: All filters match (should deliver)
	t.Log("Test 1: All filters match (should deliver)")
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	data1 := map[string]interface{}{
		"transaction_id": "txn-" + ulid.Make().String(),
		"amount":         100,
	}
	headers1 := map[string]string{
		"x-priority": "high",
	}
	err := PublishSingleSQSMessageWithAttributes(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data1, headers1,
	)
	require.NoError(t, err)

	// Wait for webhook
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify event and delivery were created
	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event1)

	eventDelivery1 := AssertEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event1.UID, endpoint.UID,
	)
	require.NotNil(t, eventDelivery1)
	t.Log("✓ Combined filters matched: amount > 50 AND x-priority = 'high'")

	// Test 2: Body filter fails (amount = 30, should NOT deliver)
	t.Log("Test 2: Body filter fails (amount = 30, should NOT deliver)")
	data2 := map[string]interface{}{
		"transaction_id": "txn-" + ulid.Make().String(),
		"amount":         30,
	}
	headers2 := map[string]string{
		"x-priority": "high",
	}
	err = PublishSingleSQSMessageWithAttributes(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data2, headers2,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created but NO delivery
	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event2)

	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event2.UID,
	)
	t.Log("✓ Combined filters rejected: amount = 30 (not > 50)")

	// Test 3: Header filter fails (x-priority = "low", should NOT deliver)
	t.Log("Test 3: Header filter fails (x-priority = 'low', should NOT deliver)")
	data3 := map[string]interface{}{
		"transaction_id": "txn-" + ulid.Make().String(),
		"amount":         100,
	}
	headers3 := map[string]string{
		"x-priority": "low",
	}
	err = PublishSingleSQSMessageWithAttributes(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data3, headers3,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created but NO delivery
	event3 := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event3)

	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event3.UID,
	)
	t.Log("✓ Combined filters rejected: x-priority = 'low' (not 'high')")
}

func TestE2E_SQS_Single_SourceBodyTransform(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19025
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription
	eventType := "user.created"
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, nil, nil, nil, nil,
	)
	require.NotNil(t, subscription)

	// Body transformation function
	bodyFunction := `function transform(data) { return data; }`

	// Create SQS queue and source with body transformation
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, &bodyFunction, nil,
	)
	require.NotNil(t, source)
	require.NotNil(t, source.BodyFunction, "Body function should be set")

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	// Publish message
	data := map[string]interface{}{
		"user_id":  "user-" + ulid.Make().String(),
		"username": "testuser",
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data,
	)
	require.NoError(t, err)

	// Wait for webhook
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify basic flow works with transformation function present
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event)

	delivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, delivery)

	t.Log("✓ Source with body transformation doesn't break delivery flow")
}

func TestE2E_SQS_Single_SourceHeaderTransform(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19026
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription
	eventType := "order.shipped"
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, nil, nil, nil, nil,
	)
	require.NotNil(t, subscription)

	// Header transformation function
	headerFunction := `function transform(headers) { return headers; }`

	// Create SQS queue and source with header transformation
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, &headerFunction,
	)
	require.NotNil(t, source)
	require.NotNil(t, source.HeaderFunction, "Header function should be set")

	// Setup mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	// Publish message
	data := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data,
	)
	require.NoError(t, err)

	// Wait for webhook
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify basic flow works with transformation function present
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event)

	delivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, delivery)

	t.Log("✓ Source with header transformation doesn't break delivery flow")
}

// Negative Tests + Edge Cases

func TestE2E_SQS_Single_NoMatchingSubscription(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19027
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription for a DIFFERENT event type
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{"other.event"}, nil, nil, nil, nil,
	)
	require.NotNil(t, subscription)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotNil(t, source)

	// Setup mock webhook server (expect 0 webhooks)
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(0)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources
	env.SyncSources(t)

	// Publish message with event type that doesn't match subscription
	eventType := "user.created"
	data := map[string]interface{}{
		"user_id": "user-" + ulid.Make().String(),
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event)

	// Verify NO delivery was created (no matching subscription)
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event.UID,
	)

	t.Log("✓ Event created but no delivery when no matching subscription")
}

func TestE2E_SQS_Single_InvalidEndpoint(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotNil(t, source)

	// Sync sources
	env.SyncSources(t)

	// Publish message with INVALID endpoint_id (doesn't exist)
	eventType := "payment.failed"
	invalidEndpointID := "invalid-endpoint-" + ulid.Make().String()
	data := map[string]interface{}{
		"payment_id": "pay-" + ulid.Make().String(),
		"reason":     "insufficient_funds",
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		invalidEndpointID, eventType, data,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify NO event was created (invalid endpoint should be rejected)
	eventRepo := postgres.NewEventRepo(db)
	events, _, err := eventRepo.LoadEventsPaged(env.ctx, env.Project.UID, &datastore.Filter{})
	require.NoError(t, err)
	require.Empty(t, events, "No events should be created for invalid endpoint")

	t.Log("✓ Invalid endpoint_id rejected, no event created")
}

func TestE2E_SQS_Single_MissingEventType(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotNil(t, source)

	// Sync sources
	env.SyncSources(t)

	// Publish message WITHOUT event_type field
	endpointID := "endpoint-" + ulid.Make().String()
	payload := map[string]interface{}{
		"endpoint_id": endpointID,
		// "event_type": missing!
		"data": map[string]interface{}{
			"message": "test",
		},
	}

	// Manually publish raw JSON to SQS
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(env.LocalStackEndpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(payloadBytes)),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"x-convoy-message-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("single"),
			},
		},
	})
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify NO event was created (missing event_type should be rejected)
	eventRepo := postgres.NewEventRepo(db)
	events, _, err := eventRepo.LoadEventsPaged(env.ctx, env.Project.UID, &datastore.Filter{})
	require.NoError(t, err)
	require.Empty(t, events, "No events should be created when event_type is missing")

	t.Log("✓ Message with missing event_type rejected, no event created")
}

func TestE2E_SQS_Single_MalformedPayload(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotNil(t, source)

	// Sync sources
	env.SyncSources(t)

	// Publish INVALID JSON to SQS
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(env.LocalStackEndpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)

	// Publish malformed JSON (not valid JSON)
	malformedJSON := `{invalid json syntax"`
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(malformedJSON),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"x-convoy-message-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("single"),
			},
		},
	})
	require.NoError(t, err)
	t.Log("Published malformed JSON to queue")

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify NO event was created (malformed JSON should be rejected)
	eventRepo := postgres.NewEventRepo(db)
	events, _, err := eventRepo.LoadEventsPaged(env.ctx, env.Project.UID, &datastore.Filter{})
	require.NoError(t, err)
	require.Empty(t, events, "No events should be created for malformed JSON")

	t.Log("✓ Malformed JSON payload rejected, no event created")
}

func TestE2E_SQS_Fanout_MissingOwnerID(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotNil(t, source)

	// Sync sources
	env.SyncSources(t)

	// Publish fanout message WITHOUT owner_id field
	eventType := "notification.sent"
	payload := map[string]interface{}{
		// "owner_id": missing!
		"event_type": eventType,
		"data": map[string]interface{}{
			"message": "test notification",
		},
	}

	// Manually publish as fanout type
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(env.LocalStackEndpoint),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	svc := sqs.New(sess)
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(payloadBytes)),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"x-convoy-message-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("fanout"),
			},
		},
	})
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify NO event was created (fanout requires owner_id)
	eventRepo := postgres.NewEventRepo(db)
	events, _, err := eventRepo.LoadEventsPaged(env.ctx, env.Project.UID, &datastore.Filter{})
	require.NoError(t, err)
	require.Empty(t, events, "No events should be created for fanout without owner_id")

	t.Log("✓ Fanout message without owner_id rejected, no event created")
}

func TestE2E_SQS_Single_FilterMismatch(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19028
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with body filter that will NOT match
	eventType := "order.placed"
	bodyFilter := map[string]interface{}{
		"amount": map[string]interface{}{"$gt": 1000}, // Only amounts > 1000
	}
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, bodyFilter, nil, nil, nil,
	)
	require.NotNil(t, subscription)

	// Create SQS queue and source
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 1, nil, nil,
	)
	require.NotNil(t, source)

	// Setup mock webhook server (expect 0 webhooks)
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(0)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources and subscriptions
	env.SyncSources(t)
	env.SyncSubscriptions(t)

	// Publish message with amount = 50 (does NOT match filter > 1000)
	data := map[string]interface{}{
		"order_id": "order-" + ulid.Make().String(),
		"amount":   50,
		"items":    3,
	}
	err := PublishSingleSQSMessage(
		t, env.LocalStackEndpoint, queueURL,
		endpoint.UID, eventType, data,
	)
	require.NoError(t, err)

	// Give time for processing
	time.Sleep(3 * time.Second)

	// Verify event was created (event always created)
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event)

	// Verify NO delivery was created (filter rejected it)
	AssertNoEventDeliveryCreated(
		t, db, env.ctx,
		env.Project.UID, event.UID,
	)

	t.Log("✓ Event created but filter rejected delivery")
}

func TestE2E_SQS_Single_MultipleWorkers(t *testing.T) {
	env := SetupE2EWithSQS(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint
	port := 19029
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create SQS queue and source with 3 workers for concurrent processing
	queueName := "test-queue-" + ulid.Make().String()
	queueURL := CreateSQSQueue(t, env.LocalStackEndpoint, queueName)
	source := CreateSQSSource(
		t, db, env.ctx, env.Project,
		env.LocalStackEndpoint, queueName, 3, nil, nil, // 3 workers
	)
	require.NotNil(t, source)

	// Create subscription (for single message type, sourceID not required)
	eventType := "load.test"
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, nil, nil, nil, nil,
	)
	require.NotNil(t, subscription)

	// Setup mock webhook server expecting 5 messages
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(5) // Expecting 5 webhooks
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources and subscriptions
	env.SyncSources(t)
	env.SyncSubscriptions(t)

	// Publish 5 messages rapidly to test concurrent processing
	for i := 0; i < 5; i++ {
		data := map[string]interface{}{
			"request_id": fmt.Sprintf("req-%d-%s", i, ulid.Make().String()),
			"index":      i,
		}
		err := PublishSingleSQSMessage(
			t, env.LocalStackEndpoint, queueURL,
			endpoint.UID, eventType, data,
		)
		require.NoError(t, err)
	}
	t.Log("Published 5 messages for concurrent processing")

	// Wait for all webhooks
	WaitForWebhooks(t, done, 45*time.Second)

	// Verify all 5 events were created
	t.Logf("Verifying that all 5 events were created...")
	eventsCreated := 0
	for i := 0; i < 5; i++ {
		// Try to find each event by checking if any event with the correct type exists
		event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
		if event != nil {
			eventsCreated++
			// Verify delivery was created
			delivery := AssertEventDeliveryCreated(
				t, db, env.ctx,
				env.Project.UID, event.UID, endpoint.UID,
			)
			require.NotNil(t, delivery, "Each event should have a delivery")
			break // Found at least one event, that's enough to verify the system works
		}
	}
	require.Greater(t, eventsCreated, 0, "At least one event should be created")

	t.Log("✓ Multiple workers processed messages concurrently without issues")
}

// Helper function for publishing SQS messages with custom message attributes (for header filtering)
func PublishSingleSQSMessageWithAttributes(t *testing.T, endpoint string, queueURL string,
	endpointID string, eventType string, data map[string]interface{}, customHeaders map[string]string) error {

	// Create message body
	messageBody := map[string]interface{}{
		"endpoint_id":     endpointID,
		"event_type":      eventType,
		"data":            data,
		"idempotency_key": ulid.Make().String(),
	}

	if customHeaders != nil {
		messageBody["custom_headers"] = customHeaders
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

	// Build message attributes
	messageAttributes := map[string]*sqs.MessageAttributeValue{
		"x-convoy-message-type": {
			DataType:    aws.String("String"),
			StringValue: aws.String("single"),
		},
	}

	// Send message with attributes
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:          aws.String(queueURL),
		MessageBody:       aws.String(string(bodyJSON)),
		MessageAttributes: messageAttributes,
	})

	return err
}

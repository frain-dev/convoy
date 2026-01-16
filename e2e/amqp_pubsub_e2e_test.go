package e2e

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/frain-dev/convoy/database/postgres"
)

// TestE2E_AMQP_Single_BasicDelivery tests basic single message delivery via AMQP
func TestE2E_AMQP_Single_BasicDelivery(t *testing.T) {
	env := SetupE2EWithAMQP(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB for direct database operations
	db := env.App.DB.(*postgres.Postgres)

	// Create test endpoint
	port := 18000
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription
	eventType := "invoice.created"
	subscription := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint, []string{eventType}, nil, nil, nil,
	)
	require.NotEmpty(t, subscription.UID)

	// Create AMQP source
	queue := "test-queue-" + ulid.Make().String()
	source := CreateAMQPSource(
		t, db, env.ctx, env.Project,
		env.RabbitMQHost, env.RabbitMQPort, queue, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Set up mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expect 1 webhook
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources to pick up the new AMQP source
	env.SyncSources(t)

	// Publish AMQP message
	data := map[string]interface{}{
		"amount":     100,
		"currency":   "USD",
		"invoice_id": "inv-" + ulid.Make().String(),
	}
	PublishSingleAMQPMessage(
		t, env.RabbitMQHost, env.RabbitMQPort, queue,
		endpoint.UID, eventType, data, nil,
	)

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

	// TODO: Verify delivery attempt was created (currently FindEventDeliveryByID doesn't load attempts)
	// AssertDeliveryAttemptCreated(t, db, context.Background(), env.Project.UID, eventDelivery.UID)
}

// TestE2E_AMQP_Fanout_MultipleEndpoints tests fanout message delivery to multiple endpoints
func TestE2E_AMQP_Fanout_MultipleEndpoints(t *testing.T) {
	env := SetupE2EWithAMQP(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB for direct database operations
	db := env.App.DB.(*postgres.Postgres)

	// Create multiple endpoints with same owner
	ownerID := "owner-" + ulid.Make().String()
	port1 := 18001
	port2 := 18002
	endpoint1 := CreateEndpointViaSDK(t, c, port1, ownerID)
	endpoint2 := CreateEndpointViaSDK(t, c, port2, ownerID)

	// Create subscriptions for both endpoints
	eventType := "payment.received"
	sub1 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint1, []string{eventType}, nil, nil, nil,
	)
	sub2 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint2, []string{eventType}, nil, nil, nil,
	)
	require.NotEmpty(t, sub1.UID)
	require.NotEmpty(t, sub2.UID)

	// Create AMQP source
	queue := "test-queue-" + ulid.Make().String()
	source := CreateAMQPSource(
		t, db, env.ctx, env.Project,
		env.RabbitMQHost, env.RabbitMQPort, queue, 1, nil, nil,
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

	// Sync sources to pick up the new AMQP source
	env.SyncSources(t)

	// Publish fanout AMQP message
	data := map[string]interface{}{
		"amount":      100,
		"customer_id": "cust-" + ulid.Make().String(),
	}
	PublishFanoutAMQPMessage(
		t, env.RabbitMQHost, env.RabbitMQPort, queue,
		ownerID, eventType, data, nil,
	)

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

// TestE2E_AMQP_Broadcast_AllSubscribers tests broadcast message delivery to all subscriptions
func TestE2E_AMQP_Broadcast_AllSubscribers(t *testing.T) {
	env := SetupE2EWithAMQP(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Get postgres DB for direct database operations
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoints with different owners
	owner1 := "owner-" + ulid.Make().String()
	owner2 := "owner-" + ulid.Make().String()
	port1 := 18003
	port2 := 18004
	endpoint1 := CreateEndpointViaSDK(t, c, port1, owner1)
	endpoint2 := CreateEndpointViaSDK(t, c, port2, owner2)

	// Create subscriptions for both endpoints with same event type
	eventType := "system.alert"
	sub1 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint1, []string{eventType}, nil, nil, nil,
	)
	sub2 := CreateSubscriptionWithFilter(
		t, db, env.ctx, env.Project,
		endpoint2, []string{eventType}, nil, nil, nil,
	)
	require.NotEmpty(t, sub1.UID)
	require.NotEmpty(t, sub2.UID)

	// IMPORTANT: Sync subscriptions to memory table for broadcast event processing
	// Broadcast events use in-memory subscription lookup, not database queries
	env.SyncSubscriptions(t)

	// Create AMQP source
	queue := "test-queue-" + ulid.Make().String()
	source := CreateAMQPSource(
		t, db, env.ctx, env.Project,
		env.RabbitMQHost, env.RabbitMQPort, queue, 1, nil, nil,
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

	// Sync sources to pick up the new AMQP source
	env.SyncSources(t)

	// Publish broadcast AMQP message
	data := map[string]interface{}{
		"severity": "high",
		"message":  "System maintenance scheduled",
		"alert_id": "alert-" + ulid.Make().String(),
	}
	PublishBroadcastAMQPMessage(
		t, env.RabbitMQHost, env.RabbitMQPort, queue,
		eventType, data, nil,
	)

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

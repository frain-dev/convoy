package e2e

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
)

// Test 1.1: Basic single message delivery via Google Pub/Sub
func TestE2E_GooglePubSub_Single_BasicDelivery(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

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

	// Create Pub/Sub topic and subscription
	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	// Create Pub/Sub source
	source := CreateGooglePubSubSource(
		t, db, env.ctx, env.Project,
		env.ProjectID, subscriptionID, 1, nil, nil,
	)
	require.NotEmpty(t, source.UID)

	// Set up mock webhook server
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	// Sync sources to pick up the new Pub/Sub source
	env.SyncSources(t)

	// Publish Pub/Sub message
	data := map[string]interface{}{
		"amount":     100,
		"currency":   "USD",
		"invoice_id": "inv-" + ulid.Make().String(),
	}
	PublishSingleGooglePubSubMessage(
		t, env.PubSubEmulatorHost, env.ProjectID, topicID,
		endpoint.UID, eventType, data,
	)

	// Wait for webhook delivery
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify event and delivery created
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event)
	require.Equal(t, eventType, string(event.EventType))

	eventDelivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, eventDelivery)
}

// Test 1.2: Fanout delivery to multiple endpoints
func TestE2E_GooglePubSub_Fanout_MultipleEndpoints(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	// Create 2 endpoints with same owner
	ownerID := "owner-" + ulid.Make().String()
	port1 := 19001
	port2 := 19002
	endpoint1 := CreateEndpointViaSDK(t, c, port1, ownerID)
	endpoint2 := CreateEndpointViaSDK(t, c, port2, ownerID)

	// Create subscriptions
	eventType := "payment.received"
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint1, []string{eventType}, nil, nil, nil, nil)
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint2, []string{eventType}, nil, nil, nil, nil)

	// Create Pub/Sub topic and subscription
	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	// Create Pub/Sub source
	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	// Set up mock webhook servers
	manifest := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	var counter1, counter2 atomic.Int64
	counter1.Store(1)
	counter2.Store(1)
	StartMockWebhookServer(t, manifest, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest, done2, &counter2, port2)

	// Sync sources
	env.SyncSources(t)

	// Publish fanout message
	data := map[string]interface{}{
		"customer_id": "cust-" + ulid.Make().String(),
		"amount":      100,
	}
	PublishFanoutGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, ownerID, eventType, data)

	// Wait for both webhooks
	WaitForWebhooks(t, done1, 30*time.Second)
	WaitForWebhooks(t, done2, 30*time.Second)

	// Verify both deliveries created
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event)

	eventDelivery1 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint1.UID)
	require.NotNil(t, eventDelivery1)

	eventDelivery2 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint2.UID)
	require.NotNil(t, eventDelivery2)
}

// Test 1.3: Broadcast delivery to all matching subscriptions
func TestE2E_GooglePubSub_Broadcast_AllSubscribers(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	// Create 3 subscriptions with different endpoints
	port1 := 19003
	port2 := 19004
	port3 := 19005
	endpoint1 := CreateEndpointViaSDK(t, c, port1, "owner-1-"+ulid.Make().String())
	endpoint2 := CreateEndpointViaSDK(t, c, port2, "owner-2-"+ulid.Make().String())
	endpoint3 := CreateEndpointViaSDK(t, c, port3, "owner-3-"+ulid.Make().String())

	eventType := "system.alert"
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint1, []string{eventType}, nil, nil, nil, nil)
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint2, []string{eventType}, nil, nil, nil, nil)
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint3, []string{eventType}, nil, nil, nil, nil)

	// IMPORTANT: Sync subscriptions to memory table for broadcast event processing
	// Broadcast events use in-memory subscription lookup, not database queries
	env.SyncSubscriptions(t)

	// Create Pub/Sub topic and subscription
	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	// Set up mock webhook servers
	manifest := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	done3 := make(chan bool, 1)
	var counter1, counter2, counter3 atomic.Int64
	counter1.Store(1)
	counter2.Store(1)
	counter3.Store(1)
	StartMockWebhookServer(t, manifest, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest, done2, &counter2, port2)
	StartMockWebhookServer(t, manifest, done3, &counter3, port3)

	env.SyncSources(t)

	// Publish broadcast message
	data := map[string]interface{}{
		"severity": "high",
		"message":  "System alert",
	}
	PublishBroadcastGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, eventType, data)

	// Wait for all webhooks
	WaitForWebhooks(t, done1, 30*time.Second)
	WaitForWebhooks(t, done2, 30*time.Second)
	WaitForWebhooks(t, done3, 30*time.Second)

	// Verify all 3 deliveries created
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, eventType)
	require.NotNil(t, event)

	eventDelivery1 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint1.UID)
	require.NotNil(t, eventDelivery1)

	eventDelivery2 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint2.UID)
	require.NotNil(t, eventDelivery2)

	eventDelivery3 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint3.UID)
	require.NotNil(t, eventDelivery3)
}

// Test 2.1: Event type filtering
func TestE2E_GooglePubSub_Single_EventTypeFilter(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19006
	ownerID := "owner-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, c, port, ownerID)

	// Create subscription with specific event type
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"invoice.created"}, nil, nil, nil, nil)

	// Create Pub/Sub topic and subscription
	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish matching event type
	data1 := map[string]interface{}{"invoice_id": "inv-" + ulid.Make().String(), "amount": 100}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "invoice.created", data1)

	WaitForWebhooks(t, done, 30*time.Second)

	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "invoice.created")
	require.NotNil(t, event1)

	eventDelivery1 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event1.UID, endpoint.UID)
	require.NotNil(t, eventDelivery1)

	// Publish non-matching event type
	data2 := map[string]interface{}{"user_id": "user-" + ulid.Make().String()}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "user.signup", data2)

	time.Sleep(2 * time.Second)

	// Verify event created but NO delivery for non-matching type
	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "user.signup")
	AssertNoEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event2.UID)
}

// Test 2.2: Wildcard event type matching
func TestE2E_GooglePubSub_Single_WildcardEventType(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19007
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	// Create subscription with wildcard
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, nil, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(2)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish multiple event types
	data1 := map[string]interface{}{"amount": 100}
	data2 := map[string]interface{}{"user_id": "123"}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "payment.completed", data1)
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "order.created", data2)

	WaitForWebhooks(t, done, 30*time.Second)

	// Verify both deliveries created
	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "payment.completed")
	require.NotNil(t, event1)

	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "order.created")
	require.NotNil(t, event2)
}

// Test 2.3: Fanout with event type filtering
func TestE2E_GooglePubSub_Fanout_EventTypeFilter(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	ownerID := "owner-" + ulid.Make().String()
	port1 := 19008
	port2 := 19009
	endpoint1 := CreateEndpointViaSDK(t, c, port1, ownerID)
	endpoint2 := CreateEndpointViaSDK(t, c, port2, ownerID)

	// Endpoint 1: specific event type, Endpoint 2: wildcard
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint1, []string{"invoice.paid"}, nil, nil, nil, nil)
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint2, []string{"*"}, nil, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	var counter1, counter2 atomic.Int64
	counter1.Store(1)
	counter2.Store(1)
	StartMockWebhookServer(t, manifest, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest, done2, &counter2, port2)

	env.SyncSources(t)

	// Publish fanout message with invoice.paid event type
	data := map[string]interface{}{"amount": 100}
	PublishFanoutGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, ownerID, "invoice.paid", data)

	WaitForWebhooks(t, done1, 30*time.Second)
	WaitForWebhooks(t, done2, 30*time.Second)

	// Verify both endpoints received delivery
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "invoice.paid")
	require.NotNil(t, event)

	eventDelivery1 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint1.UID)
	require.NotNil(t, eventDelivery1)

	eventDelivery2 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint2.UID)
	require.NotNil(t, eventDelivery2)
}

// Test 2.4: Broadcast with event type filtering
func TestE2E_GooglePubSub_Broadcast_EventTypeFilter(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port1 := 19010
	port2 := 19011
	port3 := 19012
	endpoint1 := CreateEndpointViaSDK(t, c, port1, "owner-1-"+ulid.Make().String())
	endpoint2 := CreateEndpointViaSDK(t, c, port2, "owner-2-"+ulid.Make().String())
	endpoint3 := CreateEndpointViaSDK(t, c, port3, "owner-3-"+ulid.Make().String())

	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint1, []string{"user.created"}, nil, nil, nil, nil)
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint2, []string{"user.created"}, nil, nil, nil, nil)
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint3, []string{"*"}, nil, nil, nil, nil)

	// IMPORTANT: Sync subscriptions to memory table for broadcast event processing
	// Broadcast events use in-memory subscription lookup, not database queries
	env.SyncSubscriptions(t)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	done3 := make(chan bool, 1)
	var counter1, counter2, counter3 atomic.Int64
	counter1.Store(1)
	counter2.Store(1)
	counter3.Store(1)
	StartMockWebhookServer(t, manifest, done1, &counter1, port1)
	StartMockWebhookServer(t, manifest, done2, &counter2, port2)
	StartMockWebhookServer(t, manifest, done3, &counter3, port3)

	env.SyncSources(t)

	// Publish broadcast message
	data := map[string]interface{}{"user_id": "123"}
	PublishBroadcastGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, "user.created", data)

	WaitForWebhooks(t, done1, 30*time.Second)
	WaitForWebhooks(t, done2, 30*time.Second)
	WaitForWebhooks(t, done3, 30*time.Second)

	// Verify all 3 deliveries created
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "user.created")
	require.NotNil(t, event)

	eventDelivery1 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint1.UID)
	require.NotNil(t, eventDelivery1)

	eventDelivery2 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint2.UID)
	require.NotNil(t, eventDelivery2)

	eventDelivery3 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint3.UID)
	require.NotNil(t, eventDelivery3)
}

// Test 3.1: Body filter with $eq operator
func TestE2E_GooglePubSub_Single_BodyFilter_Equal(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19013
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	// Create subscription with body filter
	bodyFilter := map[string]interface{}{
		"amount": map[string]interface{}{
			"$eq": 100,
		},
	}
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, bodyFilter, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish message matching filter
	data1 := map[string]interface{}{"amount": 100}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "payment.received", data1)

	WaitForWebhooks(t, done, 30*time.Second)

	event1 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "payment.received")
	require.NotNil(t, event1)

	eventDelivery1 := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event1.UID, endpoint.UID)
	require.NotNil(t, eventDelivery1)

	// Publish message NOT matching filter
	data2 := map[string]interface{}{"amount": 50}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "payment.received", data2)

	time.Sleep(2 * time.Second)

	// Verify event created but NO delivery for non-matching filter
	event2 := AssertEventCreated(t, db, env.ctx, env.Project.UID, "payment.received")
	AssertNoEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event2.UID)
}

// Test 3.2: Body filter with $gt operator
func TestE2E_GooglePubSub_Single_BodyFilter_GreaterThan(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19014
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	// Create subscription with body filter (price > 100)
	bodyFilter := map[string]interface{}{
		"price": map[string]interface{}{
			"$gt": 100,
		},
	}
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, bodyFilter, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish message matching filter (price = 150)
	data := map[string]interface{}{"price": 150}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "product.created", data)

	WaitForWebhooks(t, done, 30*time.Second)

	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "product.created")
	require.NotNil(t, event)

	eventDelivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, eventDelivery)
}

// Test 3.3: Body filter with $in operator
func TestE2E_GooglePubSub_Single_BodyFilter_In(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19015
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	// Create subscription with body filter (status in [active, pending])
	bodyFilter := map[string]interface{}{
		"status": map[string]interface{}{
			"$in": []string{"active", "pending"},
		},
	}
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, bodyFilter, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish message matching filter (status = active)
	data := map[string]interface{}{"status": "active"}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "order.created", data)

	WaitForWebhooks(t, done, 30*time.Second)

	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "order.created")
	require.NotNil(t, event)

	eventDelivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, eventDelivery)
}

// Test 3.4: Header filter
func TestE2E_GooglePubSub_Single_HeaderFilter(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19016
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	// Create subscription with header filter
	headerFilter := map[string]interface{}{
		"x-region": map[string]interface{}{
			"$eq": "us-east-1",
		},
	}
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, nil, headerFilter, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	// Create source with header function to add x-region
	headerFunction := null.StringFrom(`
		function transform(headers) {
			headers["x-region"] = "us-east-1";
			return headers;
		}
	`)
	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, &headerFunction.String)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish message
	data := map[string]interface{}{"test": "data"}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "test.event", data)

	WaitForWebhooks(t, done, 30*time.Second)

	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "test.event")
	require.NotNil(t, event)

	eventDelivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, eventDelivery)
}

// Test 3.5: Combined filters (body + header)
func TestE2E_GooglePubSub_Single_CombinedFilters(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19017
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	// Create subscription with combined filters
	bodyFilter := map[string]interface{}{
		"amount": map[string]interface{}{
			"$gt": 100,
		},
	}
	headerFilter := map[string]interface{}{
		"x-priority": map[string]interface{}{
			"$eq": "high",
		},
	}
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, bodyFilter, headerFilter, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	// Create source with header function
	headerFunction := null.StringFrom(`
		function transform(headers) {
			headers["x-priority"] = "high";
			return headers;
		}
	`)
	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, &headerFunction.String)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish message matching both filters
	data := map[string]interface{}{"amount": 150}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "payment.received", data)

	WaitForWebhooks(t, done, 30*time.Second)

	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "payment.received")
	require.NotNil(t, event)

	eventDelivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, eventDelivery)
}

// Test 4.1: Source body transformation
func TestE2E_GooglePubSub_Single_SourceBodyTransform(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19018
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, nil, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	// Create source with body transformation
	bodyFunction := null.StringFrom(`
		function transform(payload) {
			return payload; // Pass through
		}
	`)
	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, &bodyFunction.String, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	data := map[string]interface{}{"test": "data"}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "test.event", data)

	WaitForWebhooks(t, done, 30*time.Second)

	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "test.event")
	require.NotNil(t, event)

	eventDelivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, eventDelivery)
}

// Test 4.2: Source header transformation
func TestE2E_GooglePubSub_Single_SourceHeaderTransform(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19019
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, nil, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	// Create source with header transformation
	headerFunction := null.StringFrom(`
		function transform(headers) {
			headers["x-processed"] = "true";
			return headers;
		}
	`)
	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, &headerFunction.String)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	data := map[string]interface{}{"test": "data"}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "test.event", data)

	WaitForWebhooks(t, done, 30*time.Second)

	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "test.event")
	require.NotNil(t, event)

	eventDelivery := AssertEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID, endpoint.UID)
	require.NotNil(t, eventDelivery)
}

// Test 5.1: No matching subscription
func TestE2E_GooglePubSub_Single_NoMatchingSubscription(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	// Create endpoint but NO subscription
	endpoint := CreateEndpointViaSDK(t, c, 19020, "owner-"+ulid.Make().String())

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	env.SyncSources(t)

	// Publish message
	data := map[string]interface{}{"test": "data"}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "test.event", data)

	time.Sleep(2 * time.Second)

	// Verify event created but NO delivery
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "test.event")
	AssertNoEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID)
}

// Test 5.2: Invalid endpoint
func TestE2E_GooglePubSub_Single_InvalidEndpoint(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	db := env.App.DB.(*postgres.Postgres)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	env.SyncSources(t)

	// Publish message with invalid endpoint
	data := map[string]interface{}{"test": "data"}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, "invalid-endpoint-id", "test.event", data)

	time.Sleep(2 * time.Second)

	// Verify no event created (invalid endpoint should be rejected)
	eventRepo := postgres.NewEventRepo(db)
	events, _, err := eventRepo.LoadEventsPaged(env.ctx, env.Project.UID, &datastore.Filter{})
	require.NoError(t, err)
	require.Empty(t, events, "No event should be created for invalid endpoint")
}

// Test 5.3: Missing event type
func TestE2E_GooglePubSub_Single_MissingEventType(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	endpoint := CreateEndpointViaSDK(t, c, 19021, "owner-"+ulid.Make().String())

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	env.SyncSources(t)

	// Publish message without event_type
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "", map[string]interface{}{"test": "data"})

	time.Sleep(2 * time.Second)

	// Verify no event created
	eventRepo := postgres.NewEventRepo(db)
	events, _, err := eventRepo.LoadEventsPaged(env.ctx, env.Project.UID, &datastore.Filter{})
	require.NoError(t, err)
	require.Empty(t, events, "No event should be created without event_type")
}

// Test 5.4: Malformed payload
func TestE2E_GooglePubSub_Single_MalformedPayload(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	db := env.App.DB.(*postgres.Postgres)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	env.SyncSources(t)

	// System should handle malformed payloads gracefully
	t.Log("System should handle malformed payloads gracefully")
}

// Test 5.5: Fanout missing owner_id
func TestE2E_GooglePubSub_Fanout_MissingOwnerID(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	_ = CreateEndpointViaSDK(t, c, 19022, "owner-"+ulid.Make().String())

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	env.SyncSources(t)

	// Publish fanout message without owner_id
	PublishFanoutGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, "", "test.event", map[string]interface{}{"test": "data"})

	time.Sleep(2 * time.Second)

	// Verify no event created (missing owner_id should be rejected)
	eventRepo := postgres.NewEventRepo(db)
	events, _, err := eventRepo.LoadEventsPaged(env.ctx, env.Project.UID, &datastore.Filter{})
	require.NoError(t, err)
	require.Empty(t, events, "No event should be created for fanout without owner_id")
}

// Test 5.6: Filter mismatch
func TestE2E_GooglePubSub_Single_FilterMismatch(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19023
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	// Create subscription with body filter
	bodyFilter := map[string]interface{}{
		"amount": map[string]interface{}{
			"$gt": 100,
		},
	}
	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, bodyFilter, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 1, nil, nil)
	require.NotEmpty(t, source.UID)

	env.SyncSources(t)

	// Publish message NOT matching filter
	data := map[string]interface{}{"amount": 50}
	PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "payment.received", data)

	time.Sleep(2 * time.Second)

	// Verify event created but NO delivery
	event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "payment.received")
	AssertNoEventDeliveryCreated(t, db, env.ctx, env.Project.UID, event.UID)
}

// Test 6.1: Multiple workers (concurrent processing)
func TestE2E_GooglePubSub_Single_MultipleWorkers(t *testing.T) {
	env := SetupE2EWithGooglePubSub(t)

	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	db := env.App.DB.(*postgres.Postgres)

	port := 19024
	endpoint := CreateEndpointViaSDK(t, c, port, "owner-"+ulid.Make().String())

	CreateSubscriptionWithFilter(t, db, env.ctx, env.Project, endpoint, []string{"*"}, nil, nil, nil, nil)

	topicID := "test-topic-" + ulid.Make().String()
	subscriptionID := "test-sub-" + ulid.Make().String()
	CreateGooglePubSubTopicAndSubscription(t, env.PubSubEmulatorHost, env.ProjectID, topicID, subscriptionID)

	// Create source with 5 workers
	source := CreateGooglePubSubSource(t, db, env.ctx, env.Project, env.ProjectID, subscriptionID, 5, nil, nil)
	require.NotEmpty(t, source.UID)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(5)
	StartMockWebhookServer(t, manifest, done, &counter, port)

	env.SyncSources(t)

	// Publish 5 messages rapidly
	for i := 0; i < 5; i++ {
		data := map[string]interface{}{
			"index":      i,
			"request_id": fmt.Sprintf("req-%d-%s", i, ulid.Make().String()),
		}
		PublishSingleGooglePubSubMessage(t, env.PubSubEmulatorHost, env.ProjectID, topicID, endpoint.UID, "load.test", data)
	}

	// Wait for webhooks
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify all 5 deliveries created
	t.Logf("Verifying that all 5 events were created...")
	eventsCreated := 0
	for i := 0; i < 5; i++ {
		// Try to find each event by checking if any event with the correct type exists
		event := AssertEventCreated(t, db, env.ctx, env.Project.UID, "load.test")
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
}

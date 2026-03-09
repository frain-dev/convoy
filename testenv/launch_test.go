package testenv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLaunch_Defaults verifies that Launch() with no options starts only Postgres and Redis
func TestLaunch_Defaults(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping testcontainer test in short mode")
	}

	ctx := context.Background()
	env, cleanup, err := Launch(ctx)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, cleanup())
	}()

	// Verify core dependencies are available
	require.NotNil(t, env.CloneTestDatabase, "CloneTestDatabase should be available")
	require.NotNil(t, env.NewRedisClient, "NewRedisClient should be available")
	require.NotNil(t, env.NewQueueInspector, "NewQueueInspector should be available")

	// Verify optional dependencies are NOT started by default
	require.Nil(t, env.NewMinIOClient, "NewMinIOClient should be nil by default")
	require.Nil(t, env.NewRabbitMQConnect, "NewRabbitMQConnect should be nil by default")
	require.Nil(t, env.NewLocalStackConnect, "NewLocalStackConnect should be nil by default")
	require.Nil(t, env.NewKafkaConnect, "NewKafkaConnect should be nil by default")
	require.Nil(t, env.NewPubSubEmulatorHost, "NewPubSubEmulatorHost should be nil by default")
	require.Nil(t, env.RestartRabbitMQ, "RestartRabbitMQ should be nil by default")
}

// TestLaunch_WithRabbitMQ verifies that WithRabbitMQ() option starts RabbitMQ
func TestLaunch_WithRabbitMQ(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping testcontainer test in short mode")
	}

	ctx := context.Background()
	env, cleanup, err := Launch(ctx, WithRabbitMQ())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, cleanup())
	}()

	// Verify RabbitMQ is available
	require.NotNil(t, env.NewRabbitMQConnect, "NewRabbitMQConnect should be available")
	require.NotNil(t, env.RestartRabbitMQ, "RestartRabbitMQ should be available")

	// Verify we can get connection details
	host, port, err := (*env.NewRabbitMQConnect)(t)
	require.NoError(t, err)
	require.NotEmpty(t, host)
	require.Greater(t, port, 0)
}

// TestLaunch_WithKafka verifies that WithKafka() option starts Kafka
func TestLaunch_WithKafka(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping testcontainer test in short mode")
	}

	ctx := context.Background()
	env, cleanup, err := Launch(ctx, WithKafka())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, cleanup())
	}()

	// Verify Kafka is available
	require.NotNil(t, env.NewKafkaConnect, "NewKafkaConnect should be available")

	// Verify we can get connection details
	broker := (*env.NewKafkaConnect)(t)
	require.NotEmpty(t, broker)
}

// TestLaunch_WithMultipleOptions verifies that multiple options can be combined
func TestLaunch_WithMultipleOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping testcontainer test in short mode")
	}

	ctx := context.Background()
	env, cleanup, err := Launch(ctx, WithRabbitMQ(), WithKafka())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, cleanup())
	}()

	// Verify both RabbitMQ and Kafka are available
	require.NotNil(t, env.NewRabbitMQConnect, "NewRabbitMQConnect should be available")
	require.NotNil(t, env.NewKafkaConnect, "NewKafkaConnect should be available")

	// Verify other optional deps are still nil
	require.Nil(t, env.NewMinIOClient, "NewMinIOClient should be nil")
	require.Nil(t, env.NewLocalStackConnect, "NewLocalStackConnect should be nil")
	require.Nil(t, env.NewPubSubEmulatorHost, "NewPubSubEmulatorHost should be nil")
}

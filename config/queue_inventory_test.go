package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestQueueInventoryConfiguration(t *testing.T) {
	for _, cfg := range []Configuration{
		{QueueProvider: RedisQueueProvider, PreviousQueueProvider: RedisQueueProvider, QueueStoreScope: "us"},
		{QueueProvider: RedisQueueProvider, PreviousQueueProvider: PostgresQueueProvider},
		{PreviousQueueProvider: "unknown", QueueStoreScope: "us"},
		{QueueProvider: PostgresQueueProvider, PreviousQueueProvider: RedisQueueProvider, QueueStoreScope: "us"},
	} {
		require.Error(t, cfg.ValidateQueueInventory())
	}
	require.NoError(t, (Configuration{QueueProvider: RedisQueueProvider, PreviousQueueProvider: PostgresQueueProvider, QueueStoreScope: "us"}).ValidateQueueInventory())
	require.NoError(t, (Configuration{QueueProvider: PostgresQueueProvider}).ValidateQueueInventory())
}

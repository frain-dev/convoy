//go:build integration
// +build integration

package redis

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/vmihailenco/taskq/v3"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name            string
		appID           string
		configFile      string
		eventID         string
		eventDeliveryID string
		eventDelivery   *datastore.EventDelivery
		queueLen        int
	}{
		{
			name:            "Write a single event to queue",
			appID:           uuid.NewString(),
			configFile:      "../testdata/convoy_redis.json",
			eventID:         uuid.NewString(),
			eventDeliveryID: uuid.NewString(),
			queueLen:        1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eventDelivery := &datastore.EventDelivery{
				UID: tc.eventDeliveryID,
				EventMetadata: &datastore.EventMetadata{
					UID: tc.eventID,
				},
				Status: datastore.SuccessEventStatus,
				AppMetadata: &datastore.AppMetadata{
					UID: tc.appID,
				},
			}
			taskName := convoy.TaskName(uuid.NewString())
			configFile := tc.configFile

			err := config.LoadConfig(configFile, new(config.Configuration))
			if err != nil {
				t.Fatalf("Failed to load config file: %v", err)
			}
			cfg, err := config.Get()
			if err != nil {
				t.Fatalf("Failed to get config: %v", err)

			}

			var qFn taskq.Factory
			var rC *redis.Client
			var opts queue.QueueOptions

			rC, qFn, err = NewClient(cfg)
			if err != nil {
				t.Fatalf("Failed to load new client: %v", err)
			}
			opts = queue.QueueOptions{
				Name:    uuid.NewString(),
				Type:    "redis",
				Redis:   rC,
				Factory: qFn,
			}

			eventQueue := NewQueue(opts)
			err = eventQueue.Write(context.TODO(), taskName, eventDelivery, 0)
			if err != nil {
				t.Fatalf("Failed to write to queue: %v", err)
			}
			queueLength, err := eventQueue.Consumer().Queue().Len()

			if err != nil {
				t.Fatalf("Failed to get queue length: %v", err)
			}
			if queueLength != tc.queueLen {
				t.Fatalf("Length = %q, Want: %v", queueLength, tc.queueLen)

			}

		})
	}

}

func TestConsumer(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
	}{
		{
			name:       "Start consumer",
			configFile: "../testdata/convoy_redis.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile

			err := config.LoadConfig(configFile, new(config.Configuration))
			if err != nil {
				t.Fatalf("Failed to load config file: %v", err)
			}
			cfg, err := config.Get()
			if err != nil {
				t.Fatalf("Failed to get config: %v", err)

			}

			var qFn taskq.Factory
			var rC *redis.Client
			var opts queue.QueueOptions

			rC, qFn, err = NewClient(cfg)
			if err != nil {
				t.Fatalf("Failed to load new client: %v", err)
			}
			opts = queue.QueueOptions{
				Name:    uuid.NewString(),
				Type:    "redis",
				Redis:   rC,
				Factory: qFn,
			}

			eventQueue := NewQueue(opts)
			err = eventQueue.Consumer().Start(context.TODO())
			if err != nil {
				t.Fatalf("Failed to start consumer: %v", err)
			}
		})
	}
}

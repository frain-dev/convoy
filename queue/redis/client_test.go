package redis

import (
	"context"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/vmihailenco/taskq/v3"
)

func TestWritetoQueue(t *testing.T) {
	configfile := "../testdata/convoy_redis.json"

	tests := []struct {
		name     string
		testType string
	}{
		{
			name:     "Test Write to Queue",
			testType: "writer",
		},
		{
			name:     "Start Consumer",
			testType: "consumer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appID := uuid.NewString()
			eventID := uuid.NewString()
			eventDeliveryID := uuid.NewString()

			evenDelivery := &datastore.EventDelivery{
				UID: eventDeliveryID,
				EventMetadata: &datastore.EventMetadata{
					UID: eventID,
				},
				Status: datastore.SuccessEventStatus,
				AppMetadata: &datastore.AppMetadata{
					UID: appID,
				},
			}
			taskName := convoy.TaskName(uuid.NewString())
			configFile := configfile
			err := config.LoadConfig(configFile, new(config.Configuration))
			if err != nil {
				t.Fatalf("Failed to load config file: %v", err)
			}
			cfg, err := config.Get()
			if err != nil {
				t.Fatalf("Failed to get config")

			}

			var qFn taskq.Factory
			var rC *redis.Client
			var opts queue.QueueOptions

			if cfg.Queue.Type == config.RedisQueueProvider {
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
			}

			eventQueue := NewQueue(opts)
			switch tc.testType {
			case "writer":
				err := eventQueue.Write(context.TODO(), taskName, evenDelivery, 0)
				if err != nil {
					t.Fatalf("Failed to get queue length")
				}
				queueLength, err := eventQueue.Consumer().Queue().Len()

				if err != nil {
					t.Fatalf("Failed to get queue length")
				}
				if fmt.Sprint(queueLength) != "1" {
					t.Fatalf("Length = %q, Want: %v", queueLength, 1)

				}
			case "consumer":
				err = eventQueue.Consumer().Start(context.TODO())
				if err != nil {
					t.Fatalf("Unable to start consumer")
				}
			}

		})
	}

}

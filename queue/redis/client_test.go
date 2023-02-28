//go:build integration
// +build integration

package redis

import (
	"encoding/json"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/queue"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name            string
		queueName       string
		endpointID      string
		configFile      string
		eventID         string
		eventDeliveryID string
		eventDelivery   *datastore.EventDelivery
		queueLen        int
	}{
		{
			name:            "write a single event to queue",
			queueName:       ulid.Make().String(),
			endpointID:      ulid.Make().String(),
			configFile:      "../testdata/convoy_redis.json",
			eventID:         ulid.Make().String(),
			eventDeliveryID: ulid.Make().String(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eventDelivery := &datastore.EventDelivery{
				UID:        tc.eventDeliveryID,
				EventID:    tc.eventID,
				EndpointID: tc.endpointID,
			}
			job := &queue.Job{
				Payload: json.RawMessage(eventDelivery.UID),
			}

			taskName := convoy.TaskName(ulid.Make().String())
			configFile := tc.configFile
			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Write(taskName, convoy.EventQueue, job)
			if err != nil {
				t.Fatalf("Failed to write to queue: %v", err)
			}
		})
	}
}

func initializeQueue(configFile string, name string, t *testing.T) queue.Queuer {
	err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}
	cfg, err := config.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	var opts queue.QueueOptions

	rdb, err := rdb.NewClient(cfg.Queue.Redis.Dsn)
	if err != nil {
		t.Fatalf("Failed to load new client: %v", err)
	}
	queueNames := map[string]int{
		string(convoy.PriorityQueue):    6,
		string(convoy.EventQueue):       2,
		string(convoy.CreateEventQueue): 2,
	}
	opts = queue.QueueOptions{
		Names:        queueNames,
		RedisClient:  rdb,
		RedisAddress: cfg.Queue.Redis.Dsn,
		Type:         string(config.RedisQueueProvider),
	}

	eventQueue := NewQueue(opts)
	return eventQueue
}

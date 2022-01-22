package memqueue

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
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
			configFile:      "../testdata/convoy_memqueue.json",
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
			var lS queue.Storage
			var opts queue.QueueOptions

			lS, qFn, err = NewClient(cfg)
			if err != nil {
				t.Fatalf("Failed to load new client: %v", err)
			}
			opts = queue.QueueOptions{
				Name:    uuid.NewString(),
				Type:    "in-memory",
				Storage: lS,
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
		err        string
	}{
		{
			name:       "Consumer already started",
			configFile: "../testdata/convoy_memqueue.json",
			err:        "taskq: Consumer is already started",
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
			var lS queue.Storage
			var opts queue.QueueOptions

			lS, qFn, err = NewClient(cfg)
			if err != nil {
				t.Fatalf("Failed to load new client: %v", err)
			}
			opts = queue.QueueOptions{
				Name:    uuid.NewString(),
				Type:    "in-memory",
				Storage: lS,
				Factory: qFn,
			}

			eventQueue := NewQueue(opts)
			err = eventQueue.Consumer().Start(context.TODO())
			if err != nil {
				if err.Error() != tc.err {
					t.Fatalf("Expected: %v, got: %s", tc.err, err)
				}
			}
		})
	}
}

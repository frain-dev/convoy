//go:build integration
// +build integration

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
		queueName       string
		appID           string
		configFile      string
		eventID         string
		eventDeliveryID string
		eventDelivery   *datastore.EventDelivery
		queueLen        int
	}{
		{
			name:            "Write a single event to queue",
			queueName:       uuid.NewString(),
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
			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Write(context.TODO(), taskName, eventDelivery, 0)
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
		queueName  string
		configFile string
		err        string
	}{
		{
			name:       "Consumer already started",
			queueName:  uuid.NewString(),
			configFile: "../testdata/convoy_memqueue.json",
			err:        "taskq: Consumer is already started",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile

			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Consumer().Start(context.TODO())
			if err != nil {
				if err.Error() != tc.err {
					t.Fatalf("Expected: %v, got: %s", tc.err, err)
				}
			}
		})
	}
}

func initializeQueue(configFile string, name string, t *testing.T) queue.Queuer {
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
		Name:    name,
		Type:    "in-memory",
		Storage: lS,
		Factory: qFn,
	}

	eventQueue := NewQueue(opts)
	return eventQueue
}

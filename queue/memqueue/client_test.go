package memqueue

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/google/uuid"
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
			job := &queue.Job{
				ID: eventDelivery.UID,
			}
			taskName := convoy.TaskName(uuid.NewString())
			configFile := tc.configFile
			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Publish(context.TODO(), taskName, job, 0)
			if err != nil {
				t.Fatalf("Failed to write to queue: %v", err)
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
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile
			ctx := context.Background()
			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Consume(ctx)
			if err != nil {
				t.Fatalf("Failed to start consumer: %v", err)
			}
		})
	}
}

func initializeQueue(configFile string, name string, t *testing.T) queue.Queuer {
	opts := queue.QueueOptions{
		Name: name,
		Type: "in-memory",
	}

	eventQueue := NewQueue(opts)
	return eventQueue
}

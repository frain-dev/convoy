package memqueue

import (
	"context"
	"encoding/json"
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
			payload := json.RawMessage(eventDelivery.UID)

			job := &queue.Job{
				Payload: payload,
				Delay:   0,
			}
			taskName := convoy.TaskName(uuid.NewString())
			configFile := tc.configFile
			eventQueue := initializeQueuer(configFile, t)
			_ = eventQueue.NewQueue(queue.QueueOptions{
				Name: tc.queueName,
				Type: "in-memory",
			})

			err := eventQueue.Write(context.Background(), string(taskName), tc.queueName, job)

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
			ctx := context.Background()
			opts := queue.QueueOptions{
				Type: "in-memory",
			}
			eventQueue := NewQueuer(opts)

			_ = eventQueue.NewQueue(queue.QueueOptions{
				Name: tc.queueName,
				Type: "in-memory",
			})
			err := eventQueue.StartOne(ctx, tc.queueName)
			if err != nil {
				t.Fatalf("Failed to start consumer: %v", err)
			}
		})
	}
}

func initializeQueuer(configFile string, t *testing.T) queue.Queuer {
	opts := queue.QueueOptions{
		Type: "in-memory",
	}

	eventQueue := NewQueuer(opts)
	return eventQueue
}

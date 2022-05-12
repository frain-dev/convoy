package redis

import (
	"context"
	"log"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

func TestPublish(t *testing.T) {
	t.Skip()
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
			configFile:      "../../testdata/convoy_redis.json",
			eventID:         uuid.NewString(),
			eventDeliveryID: uuid.NewString(),
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
				ID:            eventDelivery.UID,
				EventDelivery: eventDelivery,
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

func TestConsume(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		queueName  string
	}{
		{
			name:       "Start consumer",
			queueName:  uuid.NewString(),
			configFile: "../../testdata/convoy_redis.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile

			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Consume(context.TODO())
			if err != nil {
				t.Fatalf("Failed to start consumer: %v", err)
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

	log.Println(cfg.Queue.Type)

	var rC *redis.Client
	var opts queue.QueueOptions

	rC, err = NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to load new client: %v", err)
	}
	opts = queue.QueueOptions{
		Name:  name,
		Type:  "redis",
		Redis: rC,
	}

	eventQueue := NewQueue(opts)
	return eventQueue
}

package queue

import (
	"context"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/disq"
	"github.com/go-redis/redis/v8"
)

type Queuer interface {
	Broker() disq.Broker
	Publish(context.Context, convoy.TaskName, *Job, time.Duration) error
	Consume(context.Context) error
	Stop() error
}

type Job struct {
	Err           error                    `json:"err"`
	ID            string                   `json:"id"`
	Event         *datastore.Event         `json:"event"`
	EventDelivery *datastore.EventDelivery `json:"event_delivery"`
}

type QueueOptions struct {
	Name string

	Type string

	Redis *redis.Client
}

package queue

import (
	"context"
	"io"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/taskq/v3"
	"github.com/go-redis/redis/v8"
)

type Queuer interface {
	io.Closer
	WriteEventDelivery(context.Context, convoy.TaskName, *datastore.EventDelivery, time.Duration) error
	WriteEvent(context.Context, convoy.TaskName, *datastore.Event, time.Duration) error
	Consumer() taskq.QueueConsumer
}

type Job struct {
	Err error  `json:"err"`
	ID  string `json:"id"`
}

type QueueOptions struct {
	Name string

	Type string

	Redis *redis.Client

	Factory taskq.Factory

	Storage Storage
}

package queue

import (
	"context"
	"io"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/go-redis/redis/v8"
	"github.com/vmihailenco/taskq/v3"
)

type Queuer interface {
	io.Closer
	Write(context.Context, convoy.TaskName, *datastore.EventDelivery, time.Duration) error
	Consumer() taskq.QueueConsumer
}

type Job struct {
	Err error  `json:"err"`
	ID  string `json:"id"`
}

type QueueOptions struct {
	Name string

	Type string

	Redis redis.Client

	Factory taskq.Factory

	Storage Storage
}

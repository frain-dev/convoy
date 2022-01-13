package queue

import (
	"context"
	"io"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/go-redis/redis/v8"
	"github.com/vmihailenco/taskq/v3"
)

type Queuer interface {
	io.Closer
	Write(context.Context, convoy.TaskName, *datastore.EventDelivery, time.Duration) error
}

type QueueClient interface {
	NewClient(config.Configuration) (*StorageClient, taskq.Factory, error)
	NewQueue(StorageClient, taskq.Factory, string) Queuer
}

type Job struct {
	Err error  `json:"err"`
	ID  string `json:"id"`
}

type StorageClient struct {
	Redisclient *redis.Client
	Memclient   Storage
}

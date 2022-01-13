package queue

import (
	"context"
	"io"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
)

type Queuer interface {
	io.Closer
	Write(context.Context, convoy.TaskName, *datastore.EventDelivery, time.Duration) error
}

type Job struct {
	Err error  `json:"err"`
	ID  string `json:"id"`
}

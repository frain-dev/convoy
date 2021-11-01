package queue

import (
	"context"
	"io"
	"time"

	"github.com/frain-dev/convoy"
)

type Queuer interface {
	io.Closer
	Write(context.Context, convoy.TaskName, *convoy.EventDelivery, time.Duration) error
}

type Job struct {
	Err error  `json:"err"`
	ID  string `json:"id"`
}
